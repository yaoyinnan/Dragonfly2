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

package rpcserver

import (
	"context"
	"reflect"
	"testing"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/plugins"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	"d7y.io/dragonfly/v2/cdn/supervisor/progress"
	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem"
	_ "d7y.io/dragonfly/v2/pkg/source/httpprotocol"
	"github.com/distribution/distribution/v3/uuid"

	// Register oss client
	_ "d7y.io/dragonfly/v2/pkg/source/ossprotocol"
)

func TestCdnSeedServer_GetPieceTasks(t *testing.T) {
	type fields struct {
		taskMgr supervisor.SeedTaskManager
		cfg     *config.Config
	}
	type args struct {
		ctx context.Context
		req *base.PieceTaskRequest
	}
	tests := []struct {
		name            string
		fields          fields
		args            args
		wantPiecePacket *base.PiecePacket
		wantErr         bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			css := &server{
				taskMgr: tt.fields.taskMgr,
				cfg:     tt.fields.cfg,
			}
			gotPiecePacket, err := css.GetPieceTasks(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPieceTasks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPiecePacket, tt.wantPiecePacket) {
				t.Errorf("GetPieceTasks() gotPiecePacket = %v, want %v", gotPiecePacket, tt.wantPiecePacket)
			}
		})
	}
}

func TestCdnSeedServer_ObtainSeeds(t *testing.T) {
	cfg := config.New()
	if err := plugins.Initialize(cfg.Plugins); err != nil {
		t.Fatal(err, "Initialize plugins")
	}
	progressMgr, err := progress.NewManager()
	if err != nil {
		t.Fatal(err, "create progress manager")
	}

	// Initialize storage manager
	storageMgr, ok := storage.Get(cfg.StorageMode)
	if !ok {
		t.Fatal(err, "create storage")
	}

	// Initialize CDN manager
	cdnMgr, err := cdn.NewManager(cfg, storageMgr, progressMgr)
	if err != nil {
		t.Fatal(err, "create cdn manager")
	}

	// Initialize task manager
	taskMgr, err := task.NewManager(cfg, cdnMgr, progressMgr)
	if err != nil {
		t.Fatal(err, "create task manager")
	}
	type fields struct {
		taskMgr supervisor.SeedTaskManager
		cfg     *config.Config
	}
	type args struct {
		ctx context.Context
		req *cdnsystem.SeedRequest
		psc chan *cdnsystem.PieceSeed
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantErr   bool
		testCount int
	}{
		{
			name: "testObtain",
			fields: fields{
				taskMgr: taskMgr,
				cfg:     cfg,
			},
			args: args{
				ctx: context.Background(),
				req: &cdnsystem.SeedRequest{
					TaskId: uuid.Generate().String(),
					Url:    "http://ant:sys@fileshare.glusterfs.svc.eu95.alipay.net/misc/d7y-test/blobs/sha256/16M",
					UrlMeta: &base.UrlMeta{
						Digest: "",
						Tag:    "",
						Range:  "",
						Filter: "",
						Header: nil,
					},
				},
				psc: make(chan *cdnsystem.PieceSeed, 4),
			},
			testCount: 1000,
		},
	}
	for _, tt := range tests {
		for i := 0; i < tt.testCount; i++ {
			t.Run(tt.name, func(t *testing.T) {
				css := &server{
					taskMgr: tt.fields.taskMgr,
					cfg:     tt.fields.cfg,
				}
				go func() {
					for range tt.args.psc {
					}
				}()
				if err := css.ObtainSeeds(tt.args.ctx, tt.args.req, tt.args.psc); (err != nil) != tt.wantErr {
					t.Fatalf("ObtainSeeds() error = %v, wantErr %v", err, tt.wantErr)
				} else {
					println("obtain success")
				}

			})
		}
	}
}
