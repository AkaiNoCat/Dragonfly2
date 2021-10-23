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
	"strconv"
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/internal/dferrors"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/structure/syncmap"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
)

var _ supervisor.SeedProgressManager = (*Manager)(nil)

type Manager struct {
	seedSubscribers      *syncmap.SyncMap
	taskPieceMetaRecords *syncmap.SyncMap
	mu                   *synclock.LockerPool
	timeout              time.Duration
	buffer               int
}

func NewManager() (supervisor.SeedProgressManager, error) {
	return &Manager{
		seedSubscribers:      syncmap.NewSyncMap(),
		taskPieceMetaRecords: syncmap.NewSyncMap(),
		mu:                   synclock.NewLockerPool(),
		timeout:              3 * time.Second,
		buffer:               4,
	}, nil
}

func (pm *Manager) InitSeedProgress(ctx context.Context, taskID string) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(config.EventInitSeedProgress)
	if _, loaded := pm.seedSubscribers.LoadOrStore(taskID, list.New()); loaded {
		logger.WithTaskID(taskID).Info("the task seedSubscribers already exist")
	}
	if _, loaded := pm.taskPieceMetaRecords.LoadOrStore(taskID, syncmap.NewSyncMap()); loaded {
		logger.WithTaskID(taskID).Info("the task taskPieceMetaRecords already exist")
	}
}

func (pm *Manager) WatchSeedProgress(ctx context.Context, task *types.SeedTask) (<-chan *types.SeedPiece, error) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(config.EventWatchSeedProgress)
	logger.Debugf("watch seed progress begin for taskID: %s", task.ID)
	pm.mu.Lock(task.ID, true)
	defer pm.mu.UnLock(task.ID, true)
	chanList, err := pm.seedSubscribers.GetAsList(task.ID)
	if err != nil {
		return nil, fmt.Errorf("get seed subscribers: %v", err)
	}
	pieceMetadataRecords, err := pm.getPieceMetaRecordsByTaskID(task.ID)
	if err != nil {
		return nil, fmt.Errorf("get piece meta records by taskID: %v", err)
	}
	ch := make(chan *types.SeedPiece, pm.buffer)
	ele := chanList.PushBack(ch)
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

// PublishPiece publish seedPiece
func (pm *Manager) PublishPiece(ctx context.Context, taskID string, record *types.SeedPiece) error {
	span := trace.SpanFromContext(ctx)
	recordBytes, _ := json.Marshal(record)
	span.AddEvent(config.EventPublishPiece, trace.WithAttributes(config.AttributeSeedPiece.String(string(recordBytes))))
	logger.Debugf("seed piece meta record %+v", record)
	pm.mu.Lock(taskID, false)
	defer pm.mu.UnLock(taskID, false)
	err := pm.addPieceMetaRecord(taskID, record)
	if err != nil {
		return fmt.Errorf("set piece meta record: %v", err)
	}
	chanList, err := pm.seedSubscribers.GetAsList(taskID)
	if err != nil {
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
	chanList, err := pm.seedSubscribers.GetAsList(taskID)
	if err != nil {
		return fmt.Errorf("get seed subscribers: %v", err)
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

func (pm *Manager) Clear(taskID string) error {
	pm.mu.Lock(taskID, false)
	defer pm.mu.UnLock(taskID, false)
	chanList, err := pm.seedSubscribers.GetAsList(taskID)
	if err != nil && errors.Cause(err) != dferrors.ErrDataNotFound {
		return errors.Wrap(err, "get seed subscribers")
	}
	if chanList != nil {
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
	err = pm.seedSubscribers.Remove(taskID)
	if err != nil && dferrors.ErrDataNotFound != errors.Cause(err) {
		return errors.Wrap(err, "clear seed subscribes")
	}
	err = pm.taskPieceMetaRecords.Remove(taskID)
	if err != nil && dferrors.ErrDataNotFound != errors.Cause(err) {
		return errors.Wrap(err, "clear piece meta records")
	}
	return nil
}

func (pm *Manager) GetPieces(ctx context.Context, taskID string) (records []*types.SeedPiece, err error) {
	return pm.getPieceMetaRecordsByTaskID(taskID)
}

// addPieceMetaRecord
func (pm *Manager) addPieceMetaRecord(taskID string, record *types.SeedPiece) error {
	pieceRecords, err := pm.taskPieceMetaRecords.GetAsMap(taskID)
	if err != nil {
		return err
	}
	return pieceRecords.Add(strconv.Itoa(int(record.PieceNum)), record)
}

// getPieceMetaRecordsByTaskID
func (pm *Manager) getPieceMetaRecordsByTaskID(taskID string) (records []*types.SeedPiece, err error) {
	pieceRecords, err := pm.taskPieceMetaRecords.GetAsMap(taskID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get piece meta records")
	}
	pieceNums := pieceRecords.ListKeyAsIntSlice()
	sort.Ints(pieceNums)
	for i := 0; i < len(pieceNums); i++ {
		v, _ := pieceRecords.Get(strconv.Itoa(pieceNums[i]))
		if value, ok := v.(*types.SeedPiece); ok {
			records = append(records, value)
		}
	}
	return records, nil
}
