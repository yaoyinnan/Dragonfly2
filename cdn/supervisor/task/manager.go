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

//go:generate mockgen -destination ./mock/mock_task_manager.go -package mock d7y.io/dragonfly/v2/cdn/supervisor/task SeedTaskManager
package task

import (
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/supervisor/gc"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"github.com/pkg/errors"
)

// Manager as an interface defines all operations against SeedTask.
// A SeedTask will store some meta info about the taskFile, pieces and something else.
// A seedTask corresponds to three files on the disk, which are identified by taskId, the data file meta file piece file
type Manager interface {

	// AddOrUpdate update existing task info for the key if present.
	// Otherwise, it stores and returns the given value.
	// The loaded result is true if the value was loaded, false if stored.
	AddOrUpdate(registerTask *SeedTask) (bool, error)

	// Get returns the task info with specified taskID, or nil if no
	// value is present.
	// The ok result indicates whether value was found in the taskManager.
	Get(taskID string) (*SeedTask, error)

	// Update the task info with specified taskID and updateTask
	Update(taskID string, updateTask *SeedTask) error

	// Exist check task existence with specified taskID.
	// returns the task info with specified taskID, or nil if no value is present.
	// The ok result indicates whether value was found in the taskManager.
	Exist(taskID string) (*SeedTask, bool)

	// Delete a task with specified taskID.
	Delete(taskID string)
}

// Ensure that manager implements the SeedTaskManager and gcExecutor interfaces
var (
	_ Manager     = (*manager)(nil)
	_ gc.Executor = (*manager)(nil)
)

var errTaskNotFound = errors.New("task is not found")

func IsTaskNotFound(err error) bool {
	return errors.Is(err, errTaskNotFound)
}

// manager is an implementation of the interface of Manager.
type manager struct {
	cfg                     *Config
	taskStore               sync.Map
	accessTimeMap           sync.Map
	taskURLUnreachableStore sync.Map
}

// NewManager returns a new Manager Object.
func NewManager(cfg Config) (Manager, error) {
	taskManager := &manager{
		cfg: &cfg,
	}
	gc.Register("task", cfg.GCInitialDelay, cfg.GCMetaInterval, taskManager)
	return taskManager, nil
}

func (tm *manager) AddOrUpdate(registerTask *SeedTask) (update bool, err error) {
	// add a new task or update a exist task
	update, err = tm.addOrUpdateTask(registerTask)
	// only update access when add task success
	if err == nil {
		tm.accessTimeMap.Store(registerTask.ID, time.Now())
	}
	return
}

func (tm *manager) Get(taskID string) (*SeedTask, error) {
	synclock.Lock(taskID, true)
	defer synclock.UnLock(taskID, true)
	// only update access when get task success
	if task, ok := tm.getTask(taskID); ok {
		tm.accessTimeMap.Store(taskID, time.Now())
		return task, nil
	}
	return nil, errTaskNotFound
}

func (tm *manager) Update(taskID string, taskInfo *SeedTask) error {
	synclock.Lock(taskID, false)
	defer synclock.UnLock(taskID, false)

	if err := tm.updateTask(taskID, taskInfo); err != nil {
		return err
	}
	// only update access when update task success
	tm.accessTimeMap.Store(taskID, time.Now())
	return nil
}

func (tm *manager) Exist(taskID string) (*SeedTask, bool) {
	return tm.getTask(taskID)
}

func (tm *manager) Delete(taskID string) {
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

func (tm *manager) GC() error {
	logger.Debugf("start the task meta gc job")
	startTime := time.Now()

	totalTaskNums := 0
	removedTaskCount := 0
	tm.accessTimeMap.Range(func(key, value interface{}) bool {
		totalTaskNums++
		taskID := key.(string)
		atime := value.(time.Time)
		if time.Since(atime) < tm.cfg.TaskExpireTime {
			return true
		}

		// gc task memory data
		logger.GcLogger.With("type", "meta").Infof("gc task: start to deal with task: %s", taskID)
		tm.Delete(taskID)
		removedTaskCount++
		return true
	})

	// slow GC detected, report it with a log warning
	if timeDuring := time.Since(startTime); timeDuring > gcTasksTimeout {
		logger.GcLogger.With("type", "meta").Warnf("gc tasks: %d cost: %.3f", removedTaskCount, timeDuring.Seconds())
	}
	logger.GcLogger.With("type", "meta").Infof("gc tasks: successfully full gc task count(%d), remainder count(%d)", removedTaskCount,
		totalTaskNums-removedTaskCount)
	return nil
}
