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
	"time"

	"github.com/pkg/errors"

	"d7y.io/dragonfly/v2/cdn/cdnutil"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
)

// addOrUpdateTask add a new task or update exist task
func (tm *Manager) addOrUpdateTask(registerTask *types.SeedTask) error {
	if unreachableTime, ok := tm.getTaskUnreachableTime(registerTask.ID); ok {
		if time.Since(unreachableTime) < tm.cfg.FailAccessInterval {
			// TODO 校验Header
			return errors.Errorf("hit unreachable resource %s cache and interval less than %d", registerTask.TaskURL, tm.cfg.FailAccessInterval)
		}
		tm.taskURLUnreachableStore.Delete(registerTask.ID)
		logger.Debugf("delete taskID: %s from unreachable url list", registerTask.ID)
	}
	if actual, loaded := tm.taskStore.LoadOrStore(registerTask.ID, registerTask); loaded {
		existTask := actual.(*types.SeedTask)
		if checkSame(existTask, registerTask) {
			return errors.Errorf("register task %v is conflict with exist task %v", registerTask, existTask)
		}
	}
	// using the existing task if it already exists corresponding to taskID
	synclock.Lock(registerTask.ID, true)
	task, ok := tm.getTask(registerTask.ID)
	if !ok {
		synclock.UnLock(registerTask.ID, true)
		return errTaskNotFound
	}
	if task.SourceFileLength != types.UnKnownSourceFileLen {
		synclock.UnLock(registerTask.ID, true)
		return nil
	}
	synclock.UnLock(registerTask.ID, true)
	synclock.Lock(registerTask.ID, false)
	defer synclock.UnLock(registerTask.ID, false)
	if task.SourceFileLength != types.UnKnownSourceFileLen {
		return nil
	}
	// get sourceContentLength with req.Header
	contentLengthRequest, err := source.NewRequestWithHeader(registerTask.RawURL, registerTask.Header)
	if err != nil {
		return errors.Wrap(err, "create content length request")
	}
	// add range info
	if stringutils.IsBlank(registerTask.Range) {
		contentLengthRequest.Header.Add(source.Range, registerTask.Range)
	}
	sourceFileLength, err := source.GetContentLength(contentLengthRequest)
	if err != nil {
		registerTask.Log().Errorf("get url (%s) content length failed: %v", registerTask.RawURL, err)
		if source.IsResourceNotReachableError(err) {
			tm.taskURLUnreachableStore.Store(registerTask, time.Now())
		}
		return err
	}
	// if not support file length header request ,return -1
	task.SourceFileLength = sourceFileLength
	task.Log().Debugf("success get file content length: %d", sourceFileLength)

	// if success to get the information successfully with the req.Header then update the task.UrlMeta to registerTask.UrlMeta.
	if registerTask.Header != nil {
		task.Header = registerTask.Header
	}

	// calculate piece size and update the PieceSize and PieceTotal
	if registerTask.PieceSize <= 0 {
		pieceSize := cdnutil.ComputePieceSize(registerTask.SourceFileLength)
		task.PieceSize = pieceSize
	}
	return nil
}

// updateTask update task
func (tm *Manager) updateTask(taskID string, updateTaskInfo *types.SeedTask) error {
	if updateTaskInfo == nil {
		return errors.New("updateTaskInfo is nil")
	}

	if stringutils.IsBlank(updateTaskInfo.CdnStatus) {
		return errors.New("status of updateTaskInfo is empty")
	}
	// get origin task
	task, ok := tm.getTask(taskID)
	if !ok {
		return errTaskNotFound
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
