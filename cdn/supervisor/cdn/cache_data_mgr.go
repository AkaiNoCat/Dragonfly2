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

package cdn

import (
	"io"
	"sort"

	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"github.com/pkg/errors"

	"d7y.io/dragonfly/v2/cdn/storedriver"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"d7y.io/dragonfly/v2/pkg/util/timeutils"
)

// metadataManager manages the meta file and piece meta file of each TaskID.
type metadataManager struct {
	storage     storage.Manager
	cacheLocker *synclock.LockerPool
}

func newMetadataManager(storageManager storage.Manager) *metadataManager {
	return &metadataManager{
		storageManager,
		synclock.NewLockerPool(),
	}
}

// writeFileMetadataByTask stores metadata of task
func (mm *metadataManager) writeFileMetadataByTask(seedTask *task.SeedTask) (*storage.FileMetadata, error) {
	mm.cacheLocker.Lock(seedTask.ID, false)
	defer mm.cacheLocker.UnLock(seedTask.ID, false)
	metadata := &storage.FileMetadata{
		TaskID:          seedTask.ID,
		TaskURL:         seedTask.TaskURL,
		PieceSize:       seedTask.PieceSize,
		SourceFileLen:   seedTask.SourceFileLength,
		AccessTime:      getCurrentTimeMillisFunc(),
		CdnFileLength:   seedTask.CdnFileLength,
		Digest:          seedTask.Digest,
		Tag:             seedTask.Tag,
		TotalPieceCount: seedTask.TotalPieceCount,
		Range:           seedTask.Range,
		Filter:          seedTask.Filter,
	}

	if err := mm.storage.WriteFileMetadata(seedTask.ID, metadata); err != nil {
		return nil, errors.Wrapf(err, "write task metadata file")
	}

	return metadata, nil
}

// updateAccessTime update access and interval
func (mm *metadataManager) updateAccessTime(taskID string, accessTime int64) error {
	mm.cacheLocker.Lock(taskID, false)
	defer mm.cacheLocker.UnLock(taskID, false)

	originMetadata, err := mm.readFileMetadata(taskID)
	if err != nil {
		return err
	}
	// access interval
	interval := accessTime - originMetadata.AccessTime
	originMetadata.Interval = interval
	if interval <= 0 {
		logger.WithTaskID(taskID).Warnf("file hit interval: %d, accessTime: %s", interval, timeutils.MillisUnixTime(accessTime))
		originMetadata.Interval = 0
	}

	originMetadata.AccessTime = accessTime

	return mm.storage.WriteFileMetadata(taskID, originMetadata)
}

func (mm *metadataManager) updateExpireInfo(taskID string, expireInfo map[string]string) error {
	mm.cacheLocker.Lock(taskID, false)
	defer mm.cacheLocker.UnLock(taskID, false)

	originMetadata, err := mm.readFileMetadata(taskID)
	if err != nil {
		return err
	}

	originMetadata.ExpireInfo = expireInfo

	return mm.storage.WriteFileMetadata(taskID, originMetadata)
}

func (mm *metadataManager) updateStatusAndResult(taskID string, metadata *storage.FileMetadata) error {
	mm.cacheLocker.Lock(taskID, false)
	defer mm.cacheLocker.UnLock(taskID, false)

	originMetadata, err := mm.readFileMetadata(taskID)
	if err != nil {
		return err
	}

	originMetadata.Finish = metadata.Finish
	originMetadata.Success = metadata.Success
	if originMetadata.Success {
		originMetadata.CdnFileLength = metadata.CdnFileLength
		originMetadata.SourceFileLen = metadata.SourceFileLen
		if metadata.TotalPieceCount > 0 {
			originMetadata.TotalPieceCount = metadata.TotalPieceCount
		}
		if !stringutils.IsBlank(metadata.SourceRealDigest) {
			originMetadata.SourceRealDigest = metadata.SourceRealDigest
		}
		if !stringutils.IsBlank(metadata.PieceMd5Sign) {
			originMetadata.PieceMd5Sign = metadata.PieceMd5Sign
		}
	}
	return mm.storage.WriteFileMetadata(taskID, originMetadata)
}

// appendPieceMetadata append piece meta info to storage
func (mm *metadataManager) appendPieceMetadata(taskID string, record *storage.PieceMetaRecord) error {
	mm.cacheLocker.Lock(taskID, false)
	defer mm.cacheLocker.UnLock(taskID, false)
	// write to the storage
	return mm.storage.AppendPieceMetadata(taskID, record)
}

// appendPieceMetadata append piece meta info to storage
func (mm *metadataManager) writePieceMetaRecords(taskID string, records []*storage.PieceMetaRecord) error {
	mm.cacheLocker.Lock(taskID, false)
	defer mm.cacheLocker.UnLock(taskID, false)
	// write to the storage
	return mm.storage.WritePieceMetaRecords(taskID, records)
}

// readPieceMetaRecords reads pieceMetaRecords from storage and without check data integrity
func (mm *metadataManager) readPieceMetaRecords(taskID string) ([]*storage.PieceMetaRecord, error) {
	mm.cacheLocker.Lock(taskID, true)
	defer mm.cacheLocker.UnLock(taskID, true)
	pieceMetaRecords, err := mm.storage.ReadPieceMetaRecords(taskID)
	if err != nil {
		return nil, errors.Wrapf(err, "read piece meta file")
	}
	// sort piece meta records by pieceNum
	sort.Slice(pieceMetaRecords, func(i, j int) bool {
		return pieceMetaRecords[i].PieceNum < pieceMetaRecords[j].PieceNum
	})
	return pieceMetaRecords, nil
}

func (mm *metadataManager) getPieceMd5Sign(taskID string) (string, []*storage.PieceMetaRecord, error) {
	mm.cacheLocker.Lock(taskID, true)
	defer mm.cacheLocker.UnLock(taskID, true)
	pieceMetaRecords, err := mm.storage.ReadPieceMetaRecords(taskID)
	if err != nil {
		return "", nil, errors.Wrapf(err, "read piece meta file")
	}
	var pieceMd5 []string
	sort.Slice(pieceMetaRecords, func(i, j int) bool {
		return pieceMetaRecords[i].PieceNum < pieceMetaRecords[j].PieceNum
	})
	for _, piece := range pieceMetaRecords {
		pieceMd5 = append(pieceMd5, piece.Md5)
	}
	return digestutils.Sha256(pieceMd5...), pieceMetaRecords, nil
}

func (mm *metadataManager) readFileMetadata(taskID string) (*storage.FileMetadata, error) {
	return mm.storage.ReadFileMetadata(taskID)
}

func (mm *metadataManager) statDownloadFile(taskID string) (*storedriver.StorageInfo, error) {
	return mm.storage.StatDownloadFile(taskID)
}

func (mm *metadataManager) readDownloadFile(taskID string) (io.ReadCloser, error) {
	return mm.storage.ReadDownloadFile(taskID)
}

func (mm *metadataManager) resetRepo(seedTask *task.SeedTask) error {
	mm.cacheLocker.Lock(seedTask.ID, false)
	defer mm.cacheLocker.UnLock(seedTask.ID, false)
	return mm.storage.ResetRepo(seedTask)
}

func (mm *metadataManager) writeDownloadFile(taskID string, offset int64, len int64, data io.Reader) error {
	return mm.storage.WriteDownloadFile(taskID, offset, len, data)
}
