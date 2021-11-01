/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cdn

import (
	"crypto/md5"
	"time"

	"context"

	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"d7y.io/dragonfly/v2/cdn/supervisor"
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/disk"   // To register diskStorage
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/hybrid" // To register hybridStorage
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem/server"

	"d7y.io/dragonfly/v2/pkg/synclock"
	"d7y.io/dragonfly/v2/pkg/util/timeutils"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/ratelimiter/limitreader"
	"d7y.io/dragonfly/v2/pkg/ratelimiter/ratelimiter"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
)

// Ensure that Manager implements the CDNManager interface
var _ supervisor.CDNManager = (*Manager)(nil)

var tracer = otel.Tracer("cdn-server")

// Manager is an implementation of the interface of CDNMgr.
type Manager struct {
	cfg             *config.Config
	cacheStore      storage.Manager
	limiter         *ratelimiter.RateLimiter
	cdnLocker       *synclock.LockerPool
	metadataManager *metadataManager
	progressMgr     supervisor.SeedProgressManager
	cdnReporter     *reporter
	detector        *cacheDetector
	writer          *cacheWriter
}

// NewManager returns a new Manager.
func NewManager(cfg *config.Config, cacheStore storage.Manager, progressMgr supervisor.SeedProgressManager) (supervisor.CDNManager, error) {
	return newManager(cfg, cacheStore, progressMgr)
}

func newManager(cfg *config.Config, cacheStore storage.Manager, progressManage supervisor.SeedProgressManager) (*Manager, error) {
	rateLimiter := ratelimiter.NewRateLimiter(ratelimiter.TransRate(int64(cfg.MaxBandwidth-cfg.SystemReservedBandwidth)), 2)
	metadataManager := newMetadataManager(cacheStore)
	cdnReporter := newReporter(progressManage)
	return &Manager{
		cfg:             cfg,
		cacheStore:      cacheStore,
		limiter:         rateLimiter,
		metadataManager: metadataManager,
		cdnReporter:     cdnReporter,
		progressMgr:     progressManage,
		detector:        newCacheDetector(metadataManager),
		writer:          newCacheWriter(cdnReporter, metadataManager, cacheStore),
		cdnLocker:       synclock.NewLockerPool(),
	}, nil
}

func (cm *Manager) TriggerCDN(ctx context.Context, task *types.SeedTask) (*types.SeedTask, error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanTriggerCDN)
	defer span.End()
	// obtain task write lock
	cm.cdnLocker.Lock(task.ID, false)
	defer cm.cdnLocker.UnLock(task.ID, false)

	var fileDigest = md5.New()
	var digestType = digestutils.Md5Hash.String()
	if !stringutils.IsBlank(task.Digest) {
		requestDigest := digestutils.Parse(task.Digest)
		digestType = requestDigest[0]
		fileDigest = digestutils.CreateHash(digestType)
	}
	// first: detect Cache
	detectResult, err := cm.detector.detectCache(ctx, task, fileDigest)
	if err != nil {
		return getUpdateTaskInfoWithStatusOnly(task, types.TaskInfoCdnStatusFailed), errors.Wrap(err, "detect task cache")
	}
	span.SetAttributes(config.AttributeCacheResult.String(detectResult.String()))
	task.Log().Infof("detects cache result: %+v", detectResult)
	// second: report detect result
	err = cm.cdnReporter.reportDetectResult(ctx, task.ID, detectResult)
	if err != nil {
		return getUpdateTaskInfoWithStatusOnly(task, types.TaskInfoCdnStatusFailed), errors.Wrapf(err, "report detect cache result")
	}
	// full cache
	if detectResult.breakPoint == -1 {
		task.Log().Infof("cache full hit on local")
		return getUpdateTaskInfo(task, types.TaskInfoCdnStatusSuccess, detectResult.fileMetadata.SourceRealDigest, detectResult.fileMetadata.PieceMd5Sign,
			detectResult.fileMetadata.SourceFileLen, detectResult.fileMetadata.CdnFileLength, detectResult.fileMetadata.TotalPieceCount), nil
	}
	server.StatSeedStart(task.ID, task.RawURL)
	start := time.Now()
	// third: start to download the source file
	var downloadSpan trace.Span
	ctx, downloadSpan = tracer.Start(ctx, config.SpanDownloadSource)
	downloadSpan.End()
	respBody, err := cm.download(ctx, task, detectResult.breakPoint)
	// download fail
	if err != nil {
		downloadSpan.RecordError(err)
		server.StatSeedFinish(task.ID, task.RawURL, false, err, start, time.Now(), 0, 0)
		return getUpdateTaskInfoWithStatusOnly(task, types.TaskInfoCdnStatusSourceError), errors.Wrap(err, "download task file data")
	}
	defer respBody.Close()
	reader := limitreader.NewLimitReaderWithLimiterAndDigest(respBody, cm.limiter, fileDigest, digestutils.Algorithms[digestType])

	// forth: write to storage
	downloadMetadata, err := cm.writer.startWriter(ctx, reader, task, detectResult.breakPoint)
	if err != nil {
		server.StatSeedFinish(task.ID, task.RawURL, false, err, start, time.Now(), downloadMetadata.backSourceLength,
			downloadMetadata.realSourceFileLength)
		return getUpdateTaskInfoWithStatusOnly(task, types.TaskInfoCdnStatusFailed), errors.Wrap(err, "write task file data")
	}
	server.StatSeedFinish(task.ID, task.RawURL, true, nil, start, time.Now(), downloadMetadata.backSourceLength,
		downloadMetadata.realSourceFileLength)
	// fifth: handle CDN result
	err = cm.handleCDNResult(task, downloadMetadata)
	if err != nil {
		return getUpdateTaskInfoWithStatusOnly(task, types.TaskInfoCdnStatusFailed), err
	}
	return getUpdateTaskInfo(task, types.TaskInfoCdnStatusSuccess, downloadMetadata.sourceRealDigest, downloadMetadata.pieceMd5Sign,
		downloadMetadata.realSourceFileLength, downloadMetadata.realCdnFileLength, downloadMetadata.totalPieceCount), nil
}

func (cm *Manager) Delete(taskID string) error {
	cm.cdnLocker.Lock(taskID, false)
	defer cm.cdnLocker.UnLock(taskID, false)
	err := cm.cacheStore.DeleteTask(taskID)
	if err != nil {
		return errors.Wrap(err, "failed to delete task files")
	}
	return nil
}

func (cm *Manager) TryFreeSpace(fileLength int64) (bool, error) {
	return cm.cacheStore.TryFreeSpace(fileLength)
}

func (cm *Manager) handleCDNResult(task *types.SeedTask, downloadMetadata *downloadMetadata) error {
	task.Log().Debugf("start handle cdn result, downloadMetadata: %+v", downloadMetadata)
	var isSuccess = true
	var err error
	// check md5
	if !stringutils.IsBlank(task.Digest) && task.Digest != downloadMetadata.sourceRealDigest {
		err = errors.Errorf("file digest not match expected: %s real: %s", task.Digest, downloadMetadata.sourceRealDigest)
		isSuccess = false
	}
	// check source length
	if isSuccess && task.SourceFileLength >= 0 && task.SourceFileLength != downloadMetadata.realSourceFileLength {
		err = errors.Errorf("file length not match expected: %d real: %d", task.SourceFileLength, downloadMetadata.realSourceFileLength)
		isSuccess = false
	}
	if isSuccess && task.TotalPieceCount > 0 && downloadMetadata.totalPieceCount != task.TotalPieceCount {
		err = errors.Errorf("task total piece count not match expected: %d real: %d", task.TotalPieceCount, downloadMetadata.totalPieceCount)
		isSuccess = false
	}
	sourceFileLen := task.SourceFileLength
	if isSuccess && task.SourceFileLength <= 0 {
		sourceFileLen = downloadMetadata.realSourceFileLength
	}
	err = cm.metadataManager.updateStatusAndResult(task.ID, &storage.FileMetadata{
		Finish:           true,
		Success:          isSuccess,
		SourceFileLen:    sourceFileLen,
		CdnFileLength:    downloadMetadata.realCdnFileLength,
		SourceRealDigest: downloadMetadata.sourceRealDigest,
		TotalPieceCount:  downloadMetadata.totalPieceCount,
		PieceMd5Sign:     downloadMetadata.pieceMd5Sign,
	})
	return err
}

func (cm *Manager) updateExpireInfo(taskID string, expireInfo map[string]string) {
	if err := cm.metadataManager.updateExpireInfo(taskID, expireInfo); err != nil {
		logger.WithTaskID(taskID).Errorf("failed to update expireInfo(%s): %v", expireInfo, err)
	}
	logger.WithTaskID(taskID).Infof("success to update expireInfo(%s)", expireInfo)
}

/*
	helper functions
*/
var getCurrentTimeMillisFunc = timeutils.CurrentTimeMillis

func getUpdateTaskInfoWithStatusOnly(task *types.SeedTask, cdnStatus string) *types.SeedTask {
	cloneTask := task.Clone()
	cloneTask.CdnStatus = cdnStatus
	return cloneTask
}

func getUpdateTaskInfo(task *types.SeedTask, cdnStatus, realMD5, pieceMd5Sign string, sourceFileLength, cdnFileLength int64,
	totalPieceCount int32) *types.SeedTask {
	cloneTask := task.Clone()
	cloneTask.SourceFileLength = sourceFileLength
	cloneTask.CdnFileLength = cdnFileLength
	cloneTask.CdnStatus = cdnStatus
	cloneTask.TotalPieceCount = totalPieceCount
	cloneTask.SourceRealDigest = realMD5
	cloneTask.PieceMd5Sign = pieceMd5Sign
	return cloneTask
}
