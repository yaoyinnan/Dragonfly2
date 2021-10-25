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
	"time"

	"d7y.io/dragonfly/v2/cdn/cdnutil"
	"d7y.io/dragonfly/v2/cdn/config"
	cdnerrors "d7y.io/dragonfly/v2/cdn/errors"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
)

// addOrUpdateTask add a new task or update exist task
func (tm *Manager) addOrUpdateTask(ctx context.Context, registerTask *types.SeedTask) error {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanAndOrUpdateTask)
	defer span.End()
	taskID := registerTask.ID
	if key, err := tm.taskURLUnReachableStore.GetAsTime(taskID); err == nil {
		if time.Since(key) < tm.cfg.FailAccessInterval {
			// TODO 校验Header
			span.AddEvent(config.EventHitUnReachableURL)
			return errors.Wrapf(cdnerrors.ErrURLNotReachable{URL: registerTask.RawURL}, "hit unReachable cache and interval less than %d", tm.cfg.FailAccessInterval)
		}
		span.AddEvent(config.EventDeleteUnReachableTask)
		tm.taskURLUnReachableStore.Delete(taskID)
		logger.Debugf("delete taskID: %s from unreachable url list", taskID)
	}
	// using the existing task if it already exists corresponding to taskID
	actualTask, loaded := tm.taskStore.LoadOrStore(taskID, registerTask)
	task := actualTask.(*types.SeedTask)
	if loaded && !checkSame(task, registerTask) {
		span.RecordError(fmt.Errorf("newTask: %+v, existTask: %+v", registerTask, task))
		return cdnerrors.ErrTaskIDDuplicate{TaskID: taskID, Cause: fmt.Errorf("newTask: %+v, existTask: %+v", registerTask, task)}
	}
	if task.SourceFileLength != types.UnKnownSourceFileLen {
		return nil
	}
	// get sourceContentLength with req.Header
	span.AddEvent(config.EventRequestSourceFileLength)
	contentLengthRequest, err := source.NewRequestWithHeader(registerTask.RawURL, registerTask.Header)
	if registerTask.Range != "" {
		contentLengthRequest.Header.Add(source.Range, registerTask.Range)
	}
	sourceFileLength, err := source.GetContentLength(contentLengthRequest)
	if err != nil {
		registerTask.Log().Errorf("get url (%s) content length failed: %v", registerTask.RawURL, err)
		if cdnerrors.IsURLNotReachable(err) {
			tm.taskURLUnReachableStore.Add(taskID, time.Now())
			return err
		}
	}
	// if not support file length header request ,return -1
	registerTask.SourceFileLength = sourceFileLength
	registerTask.Log().Debugf("success get file content length: %d", sourceFileLength)
	if registerTask.SourceFileLength > 0 {
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
		registerTask.Header = registerTask.Header
	}

	// calculate piece size and update the PieceSize and PieceTotal
	if registerTask.PieceSize <= 0 {
		pieceSize := cdnutil.ComputePieceSize(registerTask.SourceFileLength)
		registerTask.PieceSize = pieceSize
	}
	tm.taskStore.Add(registerTask.ID, registerTask)
	registerTask.Log().Debugf("success add task: %+v", registerTask)
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
	task, err := tm.getTask(taskID)
	if err != nil {
		return err
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
