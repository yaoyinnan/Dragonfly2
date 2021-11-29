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

//go:generate mockgen -destination ./mock/mock_progress_mgr.go -package mock d7y.io/dragonfly/v2/cdn/supervisor/progress SeedProgressManager

package progress

import (
	"container/list"
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/synclock"
)

// Manager as an interface defines all operations about seed progress
type Manager interface {

	// InitSeedProgress init task seed progress
	InitSeedProgress(ctx context.Context, taskID string)

	// WatchSeedProgress watch task seed progress
	WatchSeedProgress(ctx context.Context, taskID string) (<-chan *types.SeedPiece, error)

	// PublishPiece publish piece seed
	PublishPiece(ctx context.Context, taskID string, piece *types.SeedPiece) error

	// PublishTask publish task seed
	PublishTask(ctx context.Context, taskID string, task *task.SeedTask) error

	// GetPieces get pieces by taskID
	GetPieces(taskID string) (records []*types.SeedPiece, ok bool)

	// Clear meta info of task
	Clear(taskID string)
}

var _ Manager = (*manager)(nil)

var ErrDataNotFound = errors.New("data not found")

type manager struct {
	mu                   *synclock.LockerPool
	taskManager          task.Manager
	seedSubscribers      sync.Map
	taskPieceMetaRecords sync.Map
	timeout              time.Duration
	buffer               int
}

func NewManager(taskManager task.Manager) (Manager, error) {
	return &manager{
		mu:          synclock.NewLockerPool(),
		taskManager: taskManager,
		timeout:     3 * time.Second,
		buffer:      4,
	}, nil
}

func (pm *manager) InitSeedProgress(ctx context.Context, taskID string) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(config.EventInitSeedProgress)
	if _, loaded := pm.seedSubscribers.LoadOrStore(taskID, list.New()); loaded {
		logger.WithTaskID(taskID).Info("the task seedSubscribers already exist")
	}
	if _, loaded := pm.taskPieceMetaRecords.LoadOrStore(taskID, &sync.Map{}); loaded {
		logger.WithTaskID(taskID).Info("the task taskPieceMetaRecords already exist")
	}
}

func (pm *manager) WatchSeedProgress(ctx context.Context, taskID string) (<-chan *types.SeedPiece, error) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(config.EventWatchSeedProgress)
	logger.Debugf("begin watch taskID %s seed progress", taskID)
	actual, _ := pm.seedSubscribers.LoadOrStore(taskID, list.New())
	ch := make(chan *types.SeedPiece, pm.buffer)
	chanList := actual.(*list.List)
	pm.mu.Lock(taskID, false)
	defer pm.mu.UnLock(taskID, false)
	ele := chanList.PushBack(ch)
	go func() {
		sync.NewCond(sync.Mutex{}).Wait()
		for {
			select {
			case <-ctx.Done():
				close(ch)
			case:

			}
		}
		pieceMetadataRecords, ok := pm.getPieceMetaRecords(taskID)
		if !ok {
			return nil, errors.New("piece meta records of task is not found")
		}
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
	}()
	return ch, nil
}

func (pm *manager) PublishPiece(ctx context.Context, taskID string, record *types.SeedPiece) {
	span := trace.SpanFromContext(ctx)
	recordBytes, _ := json.Marshal(record)
	span.AddEvent(config.EventPublishPiece, trace.WithAttributes(config.AttributeSeedPiece.String(string(recordBytes))))
	logger.Debugf("seed piece meta record %+v", record)
	pm.addPieceMetaRecord(taskID, record)
}


chanList, ok := pm.getSeedSubscribers(taskID)
if !ok {
return fmt.Errorf("get seed subscribers: %v", err)
}
pm.mu.Lock(taskID, true)
defer pm.mu.UnLock(taskID, true)
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

func (pm *manager) Clear(taskID string) {
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

func (pm *manager) GetPieces(taskID string) (records []*types.SeedPiece, ok bool) {
	return pm.getPieceMetaRecords(taskID)
}

func (pm *manager) getSeedSubscribers(taskID string) (*list.List, bool) {
	subscribers, ok := pm.seedSubscribers.Load(taskID)
	if !ok {
		return nil, false
	}
	return subscribers.(*list.List), true
}

// addPieceMetaRecord
func (pm *manager) addPieceMetaRecord(taskID string, record *types.SeedPiece) {
	actual, _ := pm.taskPieceMetaRecords.LoadOrStore(taskID, new(sync.Map))
	pieceRecordMap := actual.(*sync.Map)
	pieceRecordMap.Store(record.PieceNum, record)
}

// getPieceMetaRecords get piece meta records by taskID
func (pm *manager) getPieceMetaRecords(taskID string) (records []*types.SeedPiece, ok bool) {
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


type ProgressObserver interface {
	Notify()
}

type ProgressSubject interface {
	AddObservers(observer ProgressObserver)
	NotifyObservers()
}