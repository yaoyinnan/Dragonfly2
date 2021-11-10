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
//go:generate mockgen -destination ./mock/mock_task_mgr.go -package mock d7y.io/dragonfly/v2/cdn/supervisor SeedTaskManager

package supervisor

import (
	"d7y.io/dragonfly/v2/cdn/types"
)

// SeedTaskManager as an interface defines all operations against SeedTask.
// A SeedTask will store some meta info about the taskFile, pieces and something else.
// A seedTask corresponds to three files on the disk, which are identified by taskId, the data file meta file piece file
type SeedTaskManager interface {

	// AddOrUpdate add or update a task corresponding to a downloaded file.
	AddOrUpdate(registerTask *types.SeedTask) error

	// Get the task Info with specified taskID.
	Get(taskID string) (*types.SeedTask, bool)

	// Update the task info with specified taskID and updateTask
	Update(taskID string, updateTask *types.SeedTask) error

	// Exist check task existence with specified taskID.
	Exist(taskID string) (*types.SeedTask, bool)

	// Delete a task with specified taskID.
	Delete(taskID string)
}
