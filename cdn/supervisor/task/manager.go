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
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/supervisor/gc"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/atomic"
)

// Ensure that Manager implements the SeedTaskManager and gcExecutor interfaces
var (
	_ supervisor.SeedTaskManager = (*Manager)(nil)
	_ gc.Executor                = (*Manager)(nil)
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

var tracer = otel.Tracer("cdn-task-manager")

// Manager is an implementation of the interface of supervisor.TaskManager.
type Manager struct {
	cfg                     *config.Config
	taskStore               sync.Map
	accessTimeMap           sync.Map
	taskURLUnreachableStore sync.Map
	cdnMgr                  supervisor.CDNManager
	progressMgr             supervisor.SeedProgressManager
}

// NewManager returns a new Manager Object.
func NewManager(cfg *config.Config, cdnMgr supervisor.CDNManager, progressMgr supervisor.SeedProgressManager) (supervisor.SeedTaskManager, error) {
	taskMgr := &Manager{
		cfg:         cfg,
		cdnMgr:      cdnMgr,
		progressMgr: progressMgr,
	}
	gc.Register("task", cfg.GCInitialDelay, cfg.GCMetaInterval, taskMgr)
	return taskMgr, nil
}

func (tm *Manager) Register(ctx context.Context, registerTask *types.SeedTask) error {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanTaskRegister)
	defer span.End()
	taskID := registerTask.ID
	// add a new task or update a exist task
	err := tm.addOrUpdateTask(ctx, registerTask)
	if err != nil {
		span.RecordError(err)
		return errors.Wrap(err, "add or update seed task")
	}
	// update accessTime for taskId
	tm.accessTimeMap.Store(registerTask.ID, time.Now())

	// trigger CDN
	if err := tm.triggerCdnSyncAction(ctx, taskID); err != nil {
		return errors.Wrapf(err, "trigger cdn")
	}
	registerTask.Log().Infof("successfully trigger cdn sync action")
	return nil
}

// triggerCdnSyncAction
func (tm *Manager) triggerCdnSyncAction(ctx context.Context, taskID string) error {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanTriggerCDNSyncAction)
	defer span.End()
	synclock.Lock(taskID, true)
	task, ok := tm.getTask(taskID)
	if !ok {
		synclock.UnLock(taskID, true)
		return errTaskNotFound
	}
	if !task.IsFrozen() {
		span.SetAttributes(config.AttributeTaskStatus.String(task.CdnStatus))
		task.Log().Infof("seedTask status is not frozen，no need trigger again, current status: %s", task.CdnStatus)
		synclock.UnLock(task.ID, true)
		return nil
	}
	synclock.UnLock(task.ID, true)

	synclock.Lock(task.ID, false)
	defer synclock.UnLock(task.ID, false)
	// reconfirm
	span.SetAttributes(config.AttributeTaskStatus.String(task.CdnStatus))
	if !task.IsFrozen() {
		task.Log().Infof("reconfirm seedTask status is not frozen, no need trigger again, current status: %s", task.CdnStatus)
		return nil
	}
	if task.IsWait() {
		tm.progressMgr.InitSeedProgress(ctx, task.ID)
		task.Log().Infof("successfully init seed progress for task")
	}
	err := tm.updateTask(task.ID, &types.SeedTask{
		CdnStatus: types.TaskInfoCdnStatusRunning,
	})
	if err != nil {
		return errors.Wrapf(err, "update task")
	}
	// triggerCDN goroutine
	go func() {
		updateTaskInfo, err := tm.cdnMgr.TriggerCDN(ctx, task.Clone())
		if err != nil {
			task.Log().Errorf("trigger cdn get error: %v", err)
		}
		err = tm.updateTask(task.ID, updateTaskInfo)
		if err != nil {
			task.Log().Errorf("failed to update task: %v", err)
		}
		go func() {
			if err := tm.progressMgr.PublishTask(ctx, task.ID, updateTaskInfo); err != nil {
				task.Log().Errorf("failed to publish task: %v", err)
			}

		}()
		task.Log().Infof("successfully update task cdn updatedTask: %+v", updateTaskInfo)
	}()
	return nil
}

func (tm *Manager) Get(taskID string) (*types.SeedTask, bool) {
	task, ok := tm.taskStore.Load(taskID)
	if !ok {
		return nil, ok
	}
	// update accessTime for taskID
	tm.accessTimeMap.Store(taskID, time.Now())
	return task.(*types.SeedTask), true
}

// Update the info of task.
func (tm *Manager) Update(taskID string, taskInfo *types.SeedTask) error {
	synclock.Lock(taskID, false)
	defer synclock.UnLock(taskID, false)

	return tm.updateTask(taskID, taskInfo)
}

func (tm *Manager) Exist(taskID string) (*types.SeedTask, bool) {
	task, ok := tm.taskStore.Load(taskID)
	return task.(*types.SeedTask), ok
}

func (tm *Manager) Delete(taskID string) {
	synclock.Lock(taskID, false)
	defer synclock.UnLock(taskID, false)
	tm.accessTimeMap.Delete(taskID)
	tm.taskURLUnreachableStore.Delete(taskID)
	tm.taskStore.Delete(taskID)
	tm.progressMgr.Clear(taskID)
}

func (tm *Manager) GetPieces(ctx context.Context, taskID string) (pieces []*types.SeedPiece, ok bool) {
	synclock.Lock(taskID, true)
	defer synclock.UnLock(taskID, true)
	return tm.progressMgr.GetPieces(ctx, taskID)
}

const (
	// gcTasksTimeout specifies the timeout for tasks gc.
	// If the actual execution time exceeds this threshold, a warning will be thrown.
	gcTasksTimeout = 2.0 * time.Second
)

func (tm *Manager) GC() error {
	logger.Debugf("start the task meta gc job")
	startTime := time.Now()

	totalTaskNums := atomic.NewInt32(0)
	removedTaskCount := atomic.NewInt32(0)
	tm.accessTimeMap.Range(func(key, value interface{}) bool {
		totalTaskNums.Inc()
		taskID := key.(string)
		atime := value.(time.Time)
		if time.Since(atime) < tm.cfg.TaskExpireTime {
			return true
		}

		// gc task memory data
		logger.GcLogger.With("type", "meta").Infof("gc task: start to deal with task: %s", taskID)
		tm.Delete(taskID)
		removedTaskCount.Inc()
		return true
	})

	// slow GC detected, report it with a log warning
	if timeDuring := time.Since(startTime); timeDuring > gcTasksTimeout {
		logger.GcLogger.With("type", "meta").Warnf("gc tasks: %d cost: %.3f", removedTaskCount.Load(), timeDuring.Seconds())
	}
	logger.GcLogger.With("type", "meta").Infof("gc tasks: successfully full gc task count(%d), remainder count(%d)", removedTaskCount.Load(),
		totalTaskNums.Load()-removedTaskCount.Load())
	return nil
}
