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

package progress

import (
	"context"
	"reflect"
	"testing"

	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"d7y.io/dragonfly/v2/pkg/synclock"
)

func TestNewManager(t *testing.T) {
	type args struct {
		taskManager task.Manager
	}
	tests := []struct {
		name    string
		args    args
		want    Manager
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewManager(tt.args.taskManager)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewManager() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_manager_PublishPiece(t *testing.T) {
	type fields struct {
		mu               *synclock.LockerPool
		taskManager      task.Manager
		seedTaskSubjects map[string]*publisher
	}
	type args struct {
		ctx    context.Context
		taskID string
		record *task.PieceInfo
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &manager{
				mu:               tt.fields.mu,
				taskManager:      tt.fields.taskManager,
				seedTaskSubjects: tt.fields.seedTaskSubjects,
			}
			if err := pm.PublishPiece(tt.args.ctx, tt.args.taskID, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("PublishPiece() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_manager_PublishTask(t *testing.T) {
	type fields struct {
		mu               *synclock.LockerPool
		taskManager      task.Manager
		seedTaskSubjects map[string]*publisher
	}
	type args struct {
		ctx      context.Context
		taskID   string
		seedTask *task.SeedTask
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &manager{
				mu:               tt.fields.mu,
				taskManager:      tt.fields.taskManager,
				seedTaskSubjects: tt.fields.seedTaskSubjects,
			}
			if err := pm.PublishTask(tt.args.ctx, tt.args.taskID, tt.args.seedTask); (err != nil) != tt.wantErr {
				t.Errorf("PublishTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_manager_WatchSeedProgress(t *testing.T) {
	type fields struct {
		mu               *synclock.LockerPool
		taskManager      task.Manager
		seedTaskSubjects map[string]*publisher
	}
	type args struct {
		ctx    context.Context
		taskID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    <-chan *task.PieceInfo
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &manager{
				mu:               tt.fields.mu,
				taskManager:      tt.fields.taskManager,
				seedTaskSubjects: tt.fields.seedTaskSubjects,
			}
			got, err := pm.WatchSeedProgress(tt.args.ctx, tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("WatchSeedProgress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WatchSeedProgress() got = %v, want %v", got, tt.want)
			}
		})
	}
}
