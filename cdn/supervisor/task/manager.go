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
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/supervisor/gc"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"go.uber.org/atomic"
)

// Ensure that Manager implements the SeedTaskManager and gcExecutor interfaces
var (
	_ supervisor.SeedTaskManager = (*Manager)(nil)
	_ gc.Executor                = (*Manager)(nil)
)

// Manager is an implementation of the interface of supervisor.TaskManager.
type Manager struct {
	cfg                     *config.Config
	taskStore               sync.Map
	accessTimeMap           sync.Map
	taskURLUnreachableStore sync.Map
}

// NewManager returns a new Manager Object.
func NewManager(cfg *config.Config) (supervisor.SeedTaskManager, error) {
	taskMgr := &Manager{
		cfg: cfg,
	}
	gc.Register("task", cfg.GCInitialDelay, cfg.GCMetaInterval, taskMgr)
	return taskMgr, nil
}

func (tm *Manager) AddOrUpdate(registerTask *types.SeedTask) error {
	// add a new task or update a exist task
	if err := tm.addOrUpdateTask(registerTask); err != nil {
		return err
	}
	// only update access when add task success
	tm.accessTimeMap.Store(registerTask.ID, time.Now())
	return nil
}

func (tm *Manager) Get(taskID string) (*types.SeedTask, bool) {
	synclock.Lock(taskID, true)
	defer synclock.UnLock(taskID, true)
	// only update access when get task success
	if task, ok := tm.getTask(taskID); ok {
		tm.accessTimeMap.Store(taskID, time.Now())
		return task, true
	}
	return nil, false
}

// Update the info of task.
func (tm *Manager) Update(taskID string, taskInfo *types.SeedTask) error {
	synclock.Lock(taskID, false)
	defer synclock.UnLock(taskID, false)

	if err := tm.updateTask(taskID, taskInfo); err != nil {
		return err
	}
	// only update access when update task success
	tm.accessTimeMap.Store(taskID, time.Now())
	return nil
}

func (tm *Manager) Exist(taskID string) (*types.SeedTask, bool) {
	return tm.getTask(taskID)
}

func (tm *Manager) Delete(taskID string) {
	synclock.Lock(taskID, false)
	defer synclock.UnLock(taskID, false)
	tm.accessTimeMap.Delete(taskID)
	tm.taskURLUnreachableStore.Delete(taskID)
	tm.taskStore.Delete(taskID)
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
