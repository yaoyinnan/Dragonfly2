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

package supervisor

import (
	"context"

	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"github.com/pkg/errors"
)

var (
	errTaskNotFound = errors.New("task not found")
	// errResourcesLacked represents a lack of resources, for example, the disk does not have enough space.
	errResourcesLacked = errors.New("resources lacked")
)

func IsResourcesLacked(err error) bool {
	return errors.Is(err, errResourcesLacked)
}

func IsTaskNotFound(err error) bool {
	return errors.Is(err, errTaskNotFound)
}

type CDNService interface {
	// RegisterTask
	RegisterTask(ctx context.Context, registerTask *types.SeedTask) (<-chan *types.SeedPiece, error)
	// GetPieces
	GetPieces(taskID string) (pieces []*types.SeedPiece, err error)

	GetTask(taskID string) (*types.SeedTask, bool)
}

type cdnService struct {
	taskMgr     SeedTaskManager
	cdnMgr      CDNManager
	progressMgr SeedProgressManager
}

func NewCDNService(taskManager SeedTaskManager, cdnManager CDNManager, progressManager SeedProgressManager) (CDNService, error) {
	return &cdnService{
		taskMgr:     taskManager,
		cdnMgr:      cdnManager,
		progressMgr: progressManager,
	}, nil
}

func (service *cdnService) RegisterTask(ctx context.Context, registerTask *types.SeedTask) (<-chan *types.SeedPiece, error) {
	if err := service.taskMgr.AddOrUpdate(registerTask); err != nil {
		return nil, err
	}
	if err := service.triggerCdnSyncAction(ctx, registerTask.ID); err != nil {
		return nil, err
	}
	pieceChan, err := service.progressMgr.WatchSeedProgress(ctx, registerTask.ID)
	if err != nil {
		return nil, err
	}
	return pieceChan, nil
}

// triggerCdnSyncAction trigger cdn sync action
func (service *cdnService) triggerCdnSyncAction(ctx context.Context, taskID string) error {
	task, ok := service.taskMgr.Get(taskID)
	if !ok {
		return errTaskNotFound
	}
	synclock.Lock(taskID, true)
	if task.SourceFileLength > 0 {
		ok, err := service.cdnMgr.TryFreeSpace(task.SourceFileLength)
		if err != nil {
			task.Log().Errorf("failed to try free space: %v", err)
		}
		if !ok {
			return errResourcesLacked
		}
	}
	if !task.IsFrozen() {
		task.Log().Infof("seedTask status is not frozen，no need trigger again, current status: %s", task.CdnStatus)
		synclock.UnLock(task.ID, true)
		return nil
	}
	synclock.UnLock(task.ID, true)

	synclock.Lock(task.ID, false)
	defer synclock.UnLock(task.ID, false)
	// reconfirm
	if !task.IsFrozen() {
		task.Log().Infof("reconfirm seedTask status is not frozen, no need trigger again, current status: %s", task.CdnStatus)
		return nil
	}
	task.CdnStatus = types.TaskInfoCdnStatusRunning
	// triggerCDN goroutine
	go func() {
		updateTaskInfo, err := service.cdnMgr.TriggerCDN(ctx, task.Clone())
		if err != nil {
			task.Log().Errorf("trigger cdn get error: %v", err)
		}
		err = service.taskMgr.Update(task.ID, updateTaskInfo)
		if err != nil {
			task.Log().Errorf("failed to update task: %v", err)
		}
		go func() {
			if err := service.progressMgr.PublishTask(ctx, task.ID, updateTaskInfo); err != nil {
				task.Log().Errorf("failed to publish task: %v", err)
			}

		}()
		task.Log().Infof("successfully update task cdn updatedTask: %+v", updateTaskInfo)
	}()
	return nil
}

func (service *cdnService) GetPieces(taskID string) (pieces []*types.SeedPiece, err error) {
	if task, ok := service.taskMgr.Get(taskID); !ok {
		return nil, errors.New("")
	}
	if pieces, ok := service.progressMgr.GetPieces(taskID); !ok {
		return nil, errors.New("")
	}
}

func (service *cdnService) GetTask(taskID string) (*types.SeedTask, bool) {
	return service.taskMgr.Get(taskID)
}

// trigger CDN
if err := c.cdnMgr.TriggerCDN(ctx, task); err != nil {
return errors.Wrapf(err, "trigger cdn")
}
if task.IsResourcesLacked(err) {
err = dferrors.Newf(dfcodes.ResourceLacked, "resources lacked for task(%s): %v", registerTask.ID, err)
span.RecordError(err)
return err
}
registerTask.Log().Infof("successfully trigger cdn sync action")
