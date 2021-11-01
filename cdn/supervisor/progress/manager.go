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
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/config"
	cdnerrors "d7y.io/dragonfly/v2/cdn/errors"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
)

var _ supervisor.SeedProgressManager = (*Manager)(nil)

type Manager struct {
	mu                   *synclock.LockerPool
	seedSubscribers      sync.Map
	taskPieceMetaRecords sync.Map
	timeout              time.Duration
	buffer               int
}

func NewManager() (supervisor.SeedProgressManager, error) {
	return &Manager{
		mu:      synclock.NewLockerPool(),
		timeout: 3 * time.Second,
		buffer:  4,
	}, nil
}

func (pm *Manager) InitSeedProgress(ctx context.Context, taskID string) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(config.EventInitSeedProgress)
	if _, loaded := pm.seedSubscribers.LoadOrStore(taskID, list.New()); loaded {
		logger.WithTaskID(taskID).Info("the task seedSubscribers already exist")
	}
	if _, loaded := pm.taskPieceMetaRecords.LoadOrStore(taskID, &sync.Map{}); loaded {
		logger.WithTaskID(taskID).Info("the task taskPieceMetaRecords already exist")
	}
}

func (pm *Manager) WatchSeedProgress(ctx context.Context, task *types.SeedTask) (<-chan *types.SeedPiece, error) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(config.EventWatchSeedProgress)
	logger.Debugf("watch seed progress begin for taskID: %s", task.ID)
	pm.mu.Lock(task.ID, true)
	defer pm.mu.UnLock(task.ID, true)
	chanList, ok := pm.getSeedSubscribers(task.ID)
	if !ok {
		return nil, errors.Wrap(cdnerrors.ErrDataNotFound, "get seed subscribers")
	}
	ch := make(chan *types.SeedPiece, pm.buffer)
	ele := chanList.PushBack(ch)
	pieceMetadataRecords, ok := pm.getPieceMetaRecords(task.ID)
	if !ok {
		return nil, errors.Wrap(cdnerrors.ErrDataNotFound, "get piece meta records")
	}
	go func(seedCh chan *types.SeedPiece, ele *list.Element) {
		for _, pieceMetaRecord := range pieceMetadataRecords {
			logger.Debugf("seed piece meta record %+v", pieceMetaRecord)
			select {
			case seedCh <- pieceMetaRecord:
			case <-time.After(pm.timeout):
			}
		}
		if task.IsDone() {
			chanList.Remove(ele)
			close(seedCh)
		}
	}(ch, ele)
	return ch, nil
}

func (pm *Manager) PublishPiece(ctx context.Context, taskID string, record *types.SeedPiece) error {
	span := trace.SpanFromContext(ctx)
	recordBytes, _ := json.Marshal(record)
	span.AddEvent(config.EventPublishPiece, trace.WithAttributes(config.AttributeSeedPiece.String(string(recordBytes))))
	logger.Debugf("seed piece meta record %+v", record)
	err := pm.addPieceMetaRecord(taskID, record)
	if err != nil {
		return errors.Wrap(err, "add piece meta record")
	}
	chanList, ok := pm.getSeedSubscribers(taskID)
	if !ok {
		return fmt.Errorf("get seed subscribers: %v", err)
	}
	var wg sync.WaitGroup
	for e := chanList.Front(); e != nil; e = e.Next() {
		wg.Add(1)
		sub := e.Value.(chan *types.SeedPiece)
		go func(sub chan *types.SeedPiece, record *types.SeedPiece) {
			defer wg.Done()
			select {
			case sub <- record:
			case <-time.After(pm.timeout):
			}

		}(sub, record)
	}
	wg.Wait()
	return nil
}

func (pm *Manager) PublishTask(ctx context.Context, taskID string, task *types.SeedTask) error {
	span := trace.SpanFromContext(ctx)
	taskBytes, _ := json.Marshal(task)
	span.AddEvent(config.EventPublishTask, trace.WithAttributes(config.AttributeSeedTask.String(string(taskBytes))))
	logger.Debugf("publish task record %+v", task)
	pm.mu.Lock(taskID, false)
	defer pm.mu.UnLock(taskID, false)
	chanList, ok := pm.getSeedSubscribers(taskID)
	if !ok {
		return errors.Wrap(cdnerrors.ErrDataNotFound, "get seed subscribers")
	}
	// unwatch
	for e := chanList.Front(); e != nil; e = e.Next() {
		chanList.Remove(e)
		sub, ok := e.Value.(chan *types.SeedPiece)
		if !ok {
			logger.Warnf("failed to convert chan seedPiece, e.Value: %v", e.Value)
			continue
		}
		close(sub)
	}
	return nil
}

func (pm *Manager) Clear(taskID string) {
	pm.mu.Lock(taskID, false)
	defer pm.mu.UnLock(taskID, false)
	chanList, ok := pm.getSeedSubscribers(taskID)
	pm.seedSubscribers.Delete(taskID)
	pm.taskPieceMetaRecords.Delete(taskID)
	if !ok {
		logger.Warnf("taskID %s not found in seedSubscribers", taskID)
		return
	}
	for e := chanList.Front(); e != nil; e = e.Next() {
		chanList.Remove(e)
		sub, ok := e.Value.(chan *types.SeedPiece)
		if !ok {
			logger.Warnf("failed to convert chan seedPiece, e.Value: %v", e.Value)
			continue
		}
		close(sub)
	}
	chanList = nil
}

func (pm *Manager) GetPieces(ctx context.Context, taskID string) (records []*types.SeedPiece, ok bool) {
	return pm.getPieceMetaRecords(taskID)
}

func (pm *Manager) getSeedSubscribers(taskID string) (*list.List, bool) {
	subscribers, ok := pm.seedSubscribers.Load(taskID)
	if !ok {
		return nil, false
	}
	return subscribers.(*list.List), true
}

// addPieceMetaRecord
func (pm *Manager) addPieceMetaRecord(taskID string, record *types.SeedPiece) error {
	pieceRecords, ok := pm.taskPieceMetaRecords.Load(taskID)
	if !ok {
		return cdnerrors.ErrDataNotFound
	}
	pieceRecordMap, ok := pieceRecords.(*sync.Map)
	if !ok {
		return cdnerrors.ErrConvertFailed
	}
	pieceRecordMap.Store(record.PieceNum, record)
	return nil
}

// getPieceMetaRecords get piece meta records by taskID
func (pm *Manager) getPieceMetaRecords(taskID string) (records []*types.SeedPiece, ok bool) {
	pieceRecords, ok := pm.taskPieceMetaRecords.Load(taskID)
	if !ok {
		return nil, false
	}
	pieceNums := make([]int, 0)
	pieceRecordMap := pieceRecords.(*sync.Map)
	pieceRecordMap.Range(func(key, value interface{}) bool {
		if v, ok := key.(int); ok {
			pieceNums = append(pieceNums, v)
			return true
		}
		return true
	})
	sort.Ints(pieceNums)
	for i := 0; i < len(pieceNums); i++ {
		v, _ := pieceRecordMap.Load(pieceNums[i])
		if value, ok := v.(*types.SeedPiece); ok {
			records = append(records, value)
		}
	}
	return records, true
}
