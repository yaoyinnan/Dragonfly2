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

//go:generate mockgen -destination ../mocks/progress/mock_progress_manager.go -package progress d7y.io/dragonfly/v2/cdn/supervisor/progress Manager

package progress

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel/trace"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/synclock"
)

// Manager as an interface defines all operations about seed progress
type Manager interface {

	// WatchSeedProgress watch task seed progress
	WatchSeedProgress(ctx context.Context, taskID string) (<-chan *task.PieceInfo, error)

	// PublishPiece publish piece seed
	PublishPiece(ctx context.Context, taskID string, piece *task.PieceInfo) error

	// PublishTask publish task seed
	PublishTask(ctx context.Context, taskID string, task *task.SeedTask) error

	// Clear meta info of task
	Clear(taskID string)
}

var _ Manager = (*manager)(nil)

type manager struct {
	mu               *synclock.LockerPool
	taskManager      task.Manager
	seedTaskSubjects map[string]*publisher
}

func NewManager(taskManager task.Manager) (Manager, error) {
	return &manager{
		mu:               synclock.NewLockerPool(),
		taskManager:      taskManager,
		seedTaskSubjects: make(map[string]*publisher),
	}, nil
}

func (pm *manager) WatchSeedProgress(ctx context.Context, taskID string) (<-chan *task.PieceInfo, error) {
	pm.mu.Lock(taskID, false)
	defer pm.mu.UnLock(taskID, false)
	span := trace.SpanFromContext(ctx)
	span.AddEvent(config.EventWatchSeedProgress)
	pieces, err := pm.taskManager.GetProgress(taskID)
	if err != nil {
		return nil, err
	}
	var progressPublisher, ok = pm.seedTaskSubjects[taskID]
	if !ok {
		pm.seedTaskSubjects[taskID] = newProgressPublisher(taskID)
	}
	observer := newProgressSubscriber(pieces)
	logger.Debugf("begin watch taskID %s seed progress", taskID)
	progressPublisher.AddSubscriber(observer)
	return observer.Receiver(), nil
}

func (pm *manager) PublishPiece(ctx context.Context, taskID string, record *task.PieceInfo) (err error) {
	span := trace.SpanFromContext(ctx)
	recordBytes, _ := json.Marshal(record)
	span.AddEvent(config.EventPublishPiece, trace.WithAttributes(config.AttributeSeedPiece.String(string(recordBytes))))
	logger.Debugf("seed piece record %+v", record)
	if err := pm.taskManager.UpdateProgress(taskID, record); err != nil {
		return err
	}
	var progressPublisher, ok = pm.seedTaskSubjects[taskID]
	if ok {
		progressPublisher.NotifySubscribers(record)
	}
	return nil
}

func (pm *manager) PublishTask(ctx context.Context, taskID string, task *task.SeedTask) error {
	span := trace.SpanFromContext(ctx)
	recordBytes, _ := json.Marshal(task)
	span.AddEvent(config.EventPublishTask, trace.WithAttributes(config.AttributeSeedTask.String(string(recordBytes))))
}

func (pm *manager) Clear(taskID string) {
	pm.mu.Lock(taskID, false)
	defer pm.mu.UnLock(taskID, false)
	//chanList, ok := pm.getSeedSubscribers(taskID)
	//pm.seedSubscribers.Delete(taskID)
	//pm.taskPieceMetaRecords.Delete(taskID)
	//if !ok {
	//	logger.Warnf("taskID %s not found in seedSubscribers", taskID)
	//	return
	//}
	//for e := chanList.Front(); e != nil; e = e.Next() {
	//	chanList.Remove(e)
	//	sub, ok := e.Value.(chan *SeedPiece)
	//	if !ok {
	//		logger.Warnf("failed to convert chan seedPiece, e.Value: %v", e.Value)
	//		continue
	//	}
	//	close(sub)
	//}
	//chanList = nil
}
