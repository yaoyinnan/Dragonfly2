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
	"reflect"
	"sync"
	"testing"
	"time"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"github.com/stretchr/testify/suite"

	"d7y.io/dragonfly/v2/pkg/rpc/base"
)

func TestTaskManagerSuite(t *testing.T) {
	suite.Run(t, new(TaskManagerTestSuite))
}

type TaskManagerTestSuite struct {
	tm *Manager
	suite.Suite
}

func TestIsSame(t *testing.T) {
	type args struct {
		task1 *SeedTask
		task2 *SeedTask
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSame(tt.args.task1, tt.args.task2); got != tt.want {
				t.Errorf("IsSame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTaskNotFound(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTaskNotFound(tt.args.err); got != tt.want {
				t.Errorf("IsTaskNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	type args struct {
		config Config
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
			got, err := NewManager(tt.args.config)
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

func TestNewSeedTask(t *testing.T) {
	type args struct {
		taskID  string
		rawURL  string
		urlMeta *base.UrlMeta
	}
	tests := []struct {
		name string
		args args
		want *SeedTask
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewSeedTask(tt.args.taskID, tt.args.rawURL, tt.args.urlMeta); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSeedTask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedTask_Clone(t *testing.T) {
	type fields struct {
		ID               string
		RawURL           string
		TaskURL          string
		SourceFileLength int64
		CdnFileLength    int64
		PieceSize        int32
		CdnStatus        string
		TotalPieceCount  int32
		SourceRealDigest string
		PieceMd5Sign     string
		Digest           string
		Tag              string
		Range            string
		Filter           string
		Header           map[string]string
		Pieces           map[uint32]*PieceInfo
		logger           *logger.SugaredLoggerOnWith
	}
	tests := []struct {
		name   string
		fields fields
		want   *SeedTask
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SeedTask{
				ID:               tt.fields.ID,
				RawURL:           tt.fields.RawURL,
				TaskURL:          tt.fields.TaskURL,
				SourceFileLength: tt.fields.SourceFileLength,
				CdnFileLength:    tt.fields.CdnFileLength,
				PieceSize:        tt.fields.PieceSize,
				CdnStatus:        tt.fields.CdnStatus,
				TotalPieceCount:  tt.fields.TotalPieceCount,
				SourceRealDigest: tt.fields.SourceRealDigest,
				PieceMd5Sign:     tt.fields.PieceMd5Sign,
				Digest:           tt.fields.Digest,
				Tag:              tt.fields.Tag,
				Range:            tt.fields.Range,
				Filter:           tt.fields.Filter,
				Header:           tt.fields.Header,
				Pieces:           tt.fields.Pieces,
				logger:           tt.fields.logger,
			}
			if got := task.Clone(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Clone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedTask_IsDone(t *testing.T) {
	type fields struct {
		ID               string
		RawURL           string
		TaskURL          string
		SourceFileLength int64
		CdnFileLength    int64
		PieceSize        int32
		CdnStatus        string
		TotalPieceCount  int32
		SourceRealDigest string
		PieceMd5Sign     string
		Digest           string
		Tag              string
		Range            string
		Filter           string
		Header           map[string]string
		Pieces           map[uint32]*PieceInfo
		logger           *logger.SugaredLoggerOnWith
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SeedTask{
				ID:               tt.fields.ID,
				RawURL:           tt.fields.RawURL,
				TaskURL:          tt.fields.TaskURL,
				SourceFileLength: tt.fields.SourceFileLength,
				CdnFileLength:    tt.fields.CdnFileLength,
				PieceSize:        tt.fields.PieceSize,
				CdnStatus:        tt.fields.CdnStatus,
				TotalPieceCount:  tt.fields.TotalPieceCount,
				SourceRealDigest: tt.fields.SourceRealDigest,
				PieceMd5Sign:     tt.fields.PieceMd5Sign,
				Digest:           tt.fields.Digest,
				Tag:              tt.fields.Tag,
				Range:            tt.fields.Range,
				Filter:           tt.fields.Filter,
				Header:           tt.fields.Header,
				Pieces:           tt.fields.Pieces,
				logger:           tt.fields.logger,
			}
			if got := task.IsDone(); got != tt.want {
				t.Errorf("IsDone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedTask_IsError(t *testing.T) {
	type fields struct {
		ID               string
		RawURL           string
		TaskURL          string
		SourceFileLength int64
		CdnFileLength    int64
		PieceSize        int32
		CdnStatus        string
		TotalPieceCount  int32
		SourceRealDigest string
		PieceMd5Sign     string
		Digest           string
		Tag              string
		Range            string
		Filter           string
		Header           map[string]string
		Pieces           map[uint32]*PieceInfo
		logger           *logger.SugaredLoggerOnWith
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SeedTask{
				ID:               tt.fields.ID,
				RawURL:           tt.fields.RawURL,
				TaskURL:          tt.fields.TaskURL,
				SourceFileLength: tt.fields.SourceFileLength,
				CdnFileLength:    tt.fields.CdnFileLength,
				PieceSize:        tt.fields.PieceSize,
				CdnStatus:        tt.fields.CdnStatus,
				TotalPieceCount:  tt.fields.TotalPieceCount,
				SourceRealDigest: tt.fields.SourceRealDigest,
				PieceMd5Sign:     tt.fields.PieceMd5Sign,
				Digest:           tt.fields.Digest,
				Tag:              tt.fields.Tag,
				Range:            tt.fields.Range,
				Filter:           tt.fields.Filter,
				Header:           tt.fields.Header,
				Pieces:           tt.fields.Pieces,
				logger:           tt.fields.logger,
			}
			if got := task.IsError(); got != tt.want {
				t.Errorf("IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedTask_IsFrozen(t *testing.T) {
	type fields struct {
		ID               string
		RawURL           string
		TaskURL          string
		SourceFileLength int64
		CdnFileLength    int64
		PieceSize        int32
		CdnStatus        string
		TotalPieceCount  int32
		SourceRealDigest string
		PieceMd5Sign     string
		Digest           string
		Tag              string
		Range            string
		Filter           string
		Header           map[string]string
		Pieces           map[uint32]*PieceInfo
		logger           *logger.SugaredLoggerOnWith
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SeedTask{
				ID:               tt.fields.ID,
				RawURL:           tt.fields.RawURL,
				TaskURL:          tt.fields.TaskURL,
				SourceFileLength: tt.fields.SourceFileLength,
				CdnFileLength:    tt.fields.CdnFileLength,
				PieceSize:        tt.fields.PieceSize,
				CdnStatus:        tt.fields.CdnStatus,
				TotalPieceCount:  tt.fields.TotalPieceCount,
				SourceRealDigest: tt.fields.SourceRealDigest,
				PieceMd5Sign:     tt.fields.PieceMd5Sign,
				Digest:           tt.fields.Digest,
				Tag:              tt.fields.Tag,
				Range:            tt.fields.Range,
				Filter:           tt.fields.Filter,
				Header:           tt.fields.Header,
				Pieces:           tt.fields.Pieces,
				logger:           tt.fields.logger,
			}
			if got := task.IsFrozen(); got != tt.want {
				t.Errorf("IsFrozen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedTask_IsSuccess(t *testing.T) {
	type fields struct {
		ID               string
		RawURL           string
		TaskURL          string
		SourceFileLength int64
		CdnFileLength    int64
		PieceSize        int32
		CdnStatus        string
		TotalPieceCount  int32
		SourceRealDigest string
		PieceMd5Sign     string
		Digest           string
		Tag              string
		Range            string
		Filter           string
		Header           map[string]string
		Pieces           map[uint32]*PieceInfo
		logger           *logger.SugaredLoggerOnWith
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SeedTask{
				ID:               tt.fields.ID,
				RawURL:           tt.fields.RawURL,
				TaskURL:          tt.fields.TaskURL,
				SourceFileLength: tt.fields.SourceFileLength,
				CdnFileLength:    tt.fields.CdnFileLength,
				PieceSize:        tt.fields.PieceSize,
				CdnStatus:        tt.fields.CdnStatus,
				TotalPieceCount:  tt.fields.TotalPieceCount,
				SourceRealDigest: tt.fields.SourceRealDigest,
				PieceMd5Sign:     tt.fields.PieceMd5Sign,
				Digest:           tt.fields.Digest,
				Tag:              tt.fields.Tag,
				Range:            tt.fields.Range,
				Filter:           tt.fields.Filter,
				Header:           tt.fields.Header,
				Pieces:           tt.fields.Pieces,
				logger:           tt.fields.logger,
			}
			if got := task.IsSuccess(); got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedTask_IsWait(t *testing.T) {
	type fields struct {
		ID               string
		RawURL           string
		TaskURL          string
		SourceFileLength int64
		CdnFileLength    int64
		PieceSize        int32
		CdnStatus        string
		TotalPieceCount  int32
		SourceRealDigest string
		PieceMd5Sign     string
		Digest           string
		Tag              string
		Range            string
		Filter           string
		Header           map[string]string
		Pieces           map[uint32]*PieceInfo
		logger           *logger.SugaredLoggerOnWith
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SeedTask{
				ID:               tt.fields.ID,
				RawURL:           tt.fields.RawURL,
				TaskURL:          tt.fields.TaskURL,
				SourceFileLength: tt.fields.SourceFileLength,
				CdnFileLength:    tt.fields.CdnFileLength,
				PieceSize:        tt.fields.PieceSize,
				CdnStatus:        tt.fields.CdnStatus,
				TotalPieceCount:  tt.fields.TotalPieceCount,
				SourceRealDigest: tt.fields.SourceRealDigest,
				PieceMd5Sign:     tt.fields.PieceMd5Sign,
				Digest:           tt.fields.Digest,
				Tag:              tt.fields.Tag,
				Range:            tt.fields.Range,
				Filter:           tt.fields.Filter,
				Header:           tt.fields.Header,
				Pieces:           tt.fields.Pieces,
				logger:           tt.fields.logger,
			}
			if got := task.IsWait(); got != tt.want {
				t.Errorf("IsWait() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedTask_Log(t *testing.T) {
	type fields struct {
		ID               string
		RawURL           string
		TaskURL          string
		SourceFileLength int64
		CdnFileLength    int64
		PieceSize        int32
		CdnStatus        string
		TotalPieceCount  int32
		SourceRealDigest string
		PieceMd5Sign     string
		Digest           string
		Tag              string
		Range            string
		Filter           string
		Header           map[string]string
		Pieces           map[uint32]*PieceInfo
		logger           *logger.SugaredLoggerOnWith
	}
	tests := []struct {
		name   string
		fields fields
		want   *logger.SugaredLoggerOnWith
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &SeedTask{
				ID:               tt.fields.ID,
				RawURL:           tt.fields.RawURL,
				TaskURL:          tt.fields.TaskURL,
				SourceFileLength: tt.fields.SourceFileLength,
				CdnFileLength:    tt.fields.CdnFileLength,
				PieceSize:        tt.fields.PieceSize,
				CdnStatus:        tt.fields.CdnStatus,
				TotalPieceCount:  tt.fields.TotalPieceCount,
				SourceRealDigest: tt.fields.SourceRealDigest,
				PieceMd5Sign:     tt.fields.PieceMd5Sign,
				Digest:           tt.fields.Digest,
				Tag:              tt.fields.Tag,
				Range:            tt.fields.Range,
				Filter:           tt.fields.Filter,
				Header:           tt.fields.Header,
				Pieces:           tt.fields.Pieces,
				logger:           tt.fields.logger,
			}
			if got := task.Log(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Log() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_manager_AddOrUpdate(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		registerTask *SeedTask
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantSeedTask *SeedTask
		wantErr      bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			gotSeedTask, err := tm.AddOrUpdate(tt.args.registerTask)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddOrUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotSeedTask, tt.wantSeedTask) {
				t.Errorf("AddOrUpdate() gotSeedTask = %v, want %v", gotSeedTask, tt.wantSeedTask)
			}
		})
	}
}

func Test_manager_Exist(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *SeedTask
		want1  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			got, got1 := tm.Exist(tt.args.taskID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Exist() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Exist() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_manager_GC(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			if err := tm.GC(); (err != nil) != tt.wantErr {
				t.Errorf("GC() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_manager_Get(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *SeedTask
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			got, err := tm.Get(tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_manager_GetProgress(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[uint32]*PieceInfo
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			got, err := tm.GetProgress(tt.args.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProgress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProgress() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_manager_Update(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID   string
		taskInfo *SeedTask
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
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			if err := tm.Update(tt.args.taskID, tt.args.taskInfo); (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_manager_UpdateProgress(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID string
		info   *PieceInfo
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
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			if err := tm.UpdateProgress(tt.args.taskID, tt.args.info); (err != nil) != tt.wantErr {
				t.Errorf("UpdateProgress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_manager_getTask(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *SeedTask
		want1  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			got, got1 := tm.getTask(tt.args.taskID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTask() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getTask() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_manager_getTaskAccessTime(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   time.Time
		want1  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			got, got1 := tm.getTaskAccessTime(tt.args.taskID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTaskAccessTime() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getTaskAccessTime() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_manager_getTaskUnreachableTime(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   time.Time
		want1  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			got, got1 := tm.getTaskUnreachableTime(tt.args.taskID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTaskUnreachableTime() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getTaskUnreachableTime() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_manager_updateTask(t *testing.T) {
	type fields struct {
		config                  Config
		taskStore               sync.Map
		accessTimeMap           sync.Map
		taskURLUnreachableStore sync.Map
	}
	type args struct {
		taskID         string
		updateTaskInfo *SeedTask
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
			tm := &manager{
				config:                  tt.fields.config,
				taskStore:               tt.fields.taskStore,
				accessTimeMap:           tt.fields.accessTimeMap,
				taskURLUnreachableStore: tt.fields.taskURLUnreachableStore,
			}
			if err := tm.updateTask(tt.args.taskID, tt.args.updateTaskInfo); (err != nil) != tt.wantErr {
				t.Errorf("updateTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
