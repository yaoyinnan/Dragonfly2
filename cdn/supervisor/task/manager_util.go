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

package task

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"d7y.io/dragonfly/v2/cdn/cdnutil"
	"d7y.io/dragonfly/v2/cdn/config"
	cdnerrors "d7y.io/dragonfly/v2/cdn/errors"
	"d7y.io/dragonfly/v2/cdn/storedriver"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"d7y.io/dragonfly/v2/pkg/util/fileutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/atomic"
)

// addOrUpdateTask add a new task or update exist task
func (tm *Manager) addOrUpdateTask(ctx context.Context, registerTask *types.SeedTask) error {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanAndOrUpdateTask)
	defer span.End()
	if unreachableTime, ok := tm.getTaskUnreachableTime(registerTask.ID); ok {
		if time.Since(unreachableTime) < tm.cfg.FailAccessInterval {
			// TODO 校验Header
			span.AddEvent(config.EventHitUnreachableURL)
			return errors.Wrapf(cdnerrors.ErrURLNotReachable{URL: registerTask.RawURL}, "hit unreachable cache and interval less than %d",
				tm.cfg.FailAccessInterval)
		}
		span.AddEvent(config.EventDeleteUnReachableTask)
		tm.taskURLUnreachableStore.Delete(registerTask.ID)
		logger.Debugf("delete taskID: %s from unreachable url list", registerTask.ID)
	}
	task := registerTask
	// using the existing task if it already exists corresponding to taskID
	if existTask, ok := tm.getTask(registerTask.ID); ok {
		if !checkSame(existTask, registerTask) {
			span.RecordError(fmt.Errorf("newTask: %+v, existTask: %+v", registerTask, existTask))
			return cdnerrors.ErrTaskIDDuplicate{TaskID: registerTask.ID, Cause: fmt.Errorf("newTask: %+v, existTask: %+v", registerTask, existTask)}
		}
		task = existTask.Clone()
	}
	if task.SourceFileLength != types.UnKnownSourceFileLen {
		return nil
	}

	synclock.Lock(registerTask.ID, false)
	defer synclock.UnLock(registerTask.ID, false)
	// get sourceContentLength with req.Header
	span.AddEvent(config.EventRequestSourceFileLength)
	contentLengthRequest, err := source.NewRequestWithHeader(registerTask.RawURL, registerTask.Header)
	if err != nil {
		return errors.Wrap(err, "create content length request")
	}
	// add range info
	if registerTask.Range != "" {
		contentLengthRequest.Header.Add(source.Range, registerTask.Range)
	}
	sourceFileLength, err := source.GetContentLength(contentLengthRequest)
	if err != nil {
		registerTask.Log().Errorf("get url (%s) content length failed: %v", registerTask.RawURL, err)
		if cdnerrors.IsURLNotReachable(err) {
			tm.taskURLUnreachableStore.Store(registerTask, time.Now())
			return err
		}
	}
	// if not support file length header request ,return -1
	task.SourceFileLength = sourceFileLength
	task.Log().Debugf("success get file content length: %d", sourceFileLength)
	if task.SourceFileLength > 0 {
		tm.getReamain
		ok, err := tm.cdnMgr.TryFreeSpace(registerTask.SourceFileLength)
		if err != nil {
			registerTask.Log().Errorf("failed to try free space: %v", err)
		}
		if !ok {
			return cdnerrors.ErrResourcesLacked
		}
	}

	// if success to get the information successfully with the req.Header then update the task.UrlMeta to registerTask.UrlMeta.
	if registerTask.Header != nil {
		task.Header = registerTask.Header
	}

	// calculate piece size and update the PieceSize and PieceTotal
	if registerTask.PieceSize <= 0 {
		pieceSize := cdnutil.ComputePieceSize(registerTask.SourceFileLength)
		task.PieceSize = pieceSize
	}
	tm.taskStore.Store(task.ID, task)
	task.Log().Debugf("success add task: %+v", task)
	return nil
}

// updateTask update task
func (tm *Manager) updateTask(taskID string, updateTaskInfo *types.SeedTask) error {
	if updateTaskInfo == nil {
		return errors.Wrap(cdnerrors.ErrInvalidValue, "updateTaskInfo is nil")
	}

	if stringutils.IsBlank(updateTaskInfo.CdnStatus) {
		return errors.Wrap(cdnerrors.ErrInvalidValue, "status of task is empty")
	}
	// get origin task
	task, ok := tm.getTask(taskID)
	if !ok {
		return errors.Wrap(cdnerrors.ErrDataNotFound, "task not found")
	}

	if task.IsSuccess() && !updateTaskInfo.IsSuccess() {
		task.Log().Warnf("origin task status is success, but update task status is %s, return origin task", task.CdnStatus)
		return nil
	}

	// only update the task info when the new CDNStatus equals success
	// and the origin CDNStatus not equals success.
	if updateTaskInfo.CdnFileLength > 0 {
		task.CdnFileLength = updateTaskInfo.CdnFileLength
	}
	if !stringutils.IsBlank(updateTaskInfo.SourceRealDigest) {
		task.SourceRealDigest = updateTaskInfo.SourceRealDigest
	}

	if !stringutils.IsBlank(updateTaskInfo.PieceMd5Sign) {
		task.PieceMd5Sign = updateTaskInfo.PieceMd5Sign
	}
	if updateTaskInfo.SourceFileLength >= 0 {
		task.TotalPieceCount = updateTaskInfo.TotalPieceCount
		task.SourceFileLength = updateTaskInfo.SourceFileLength
	}
	task.CdnStatus = updateTaskInfo.CdnStatus
	return nil
}

// getTask get task from taskStore and convert it to *types.SeedTask type
func (tm *Manager) getTask(taskID string) (*types.SeedTask, bool) {
	task, ok := tm.taskStore.Load(taskID)
	if !ok {
		return nil, false
	}
	return task.(*types.SeedTask), true
}

// getTaskAccessTime get access time of task and convert it to time.Time type
func (tm *Manager) getTaskAccessTime(taskID string) (time.Time, bool) {
	access, ok := tm.accessTimeMap.Load(taskID)
	if !ok {
		return time.Time{}, false
	}
	return access.(time.Time), true
}

// getTaskUnreachableTime get unreachable time of task and convert it to time.Time type
func (tm *Manager) getTaskUnreachableTime(taskID string) (time.Time, bool) {
	unreachableTime, ok := tm.taskURLUnreachableStore.Load(taskID)
	if !ok {
		return time.Time{}, false
	}
	return unreachableTime.(time.Time), true
}


func getRemaind() {
	remainder := atomic.NewInt64(0)
	r := &storedriver.Raw{
		WalkFn: func(filePath string, info os.FileInfo, err error) error {
			if fileutils.IsRegular(filePath) {
				taskID := strings.Split(path.Base(filePath), ".")[0]
				task, exist := .Exist(taskID)
				if exist {
					var totalLen int64 = 0
					if task.CdnFileLength > 0 {
						totalLen = task.CdnFileLength
					} else {
						totalLen = task.SourceFileLength
					}
					if totalLen > 0 {
						remainder.Add(totalLen - info.Size())
					}
				}
			}
			return nil
		},
	}
}
// checkSame check task1 is same with task2
func checkSame(task1, task2 *types.SeedTask) bool {
	if task1 == task2 {
		return true
	}

	if task1.ID != task2.ID {
		return false
	}

	if task1.TaskURL != task2.TaskURL {
		return false
	}

	if task1.Range != task2.Range {
		return false
	}

	if task1.Tag != task2.Tag {
		return false
	}

	if task1.Filter != task2.Filter {
		return false
	}
	return true
}
