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
//go:generate mockgen -destination ./mock/mock_storage_manager.go -package mock d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage Manager

package storage

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"github.com/pkg/errors"

	"d7y.io/dragonfly/v2/cdn/storedriver"
	"d7y.io/dragonfly/v2/pkg/unit"
	"d7y.io/dragonfly/v2/pkg/util/rangeutils"
)

var (
	// m is a map from name to storageManager builder.
	m = make(map[string]Builder)
)

type Manager interface {

	// ResetRepo reset the storage of task
	ResetRepo(task *task.SeedTask) error

	// StatDownloadFile stat download file info
	StatDownloadFile(taskID string) (*storedriver.StorageInfo, error)

	// WriteDownloadFile write data to download file
	WriteDownloadFile(taskID string, offset int64, len int64, data io.Reader) error

	// ReadDownloadFile return reader of download file
	ReadDownloadFile(taskID string) (io.ReadCloser, error)

	// ReadFileMetadata return meta data of download file
	ReadFileMetadata(taskID string) (*FileMetadata, error)

	// WriteFileMetadata write file meta to storage
	WriteFileMetadata(taskID string, meta *FileMetadata) error

	// WritePieceMetaRecords write piece meta records to storage
	WritePieceMetaRecords(taskID string, metaRecords []*PieceMetaRecord) error

	// AppendPieceMetadata append piece meta data to storage
	AppendPieceMetadata(taskID string, metaRecord *PieceMetaRecord) error

	// ReadPieceMetaRecords read piece meta records from storage
	ReadPieceMetaRecords(taskID string) ([]*PieceMetaRecord, error)

	// DeleteTask delete task from storage
	DeleteTask(taskID string) error

	// TryFreeSpace checks if there is enough space for the file, return true while we are sure that there is enough space.
	TryFreeSpace(fileLength int64) (bool, error)
}

// FileMetadata meta data of task
type FileMetadata struct {
	TaskID           string            `json:"taskID"`
	TaskURL          string            `json:"taskURL"`
	PieceSize        int32             `json:"pieceSize"`
	SourceFileLen    int64             `json:"sourceFileLen"`
	AccessTime       int64             `json:"accessTime"`
	Interval         int64             `json:"interval"`
	CdnFileLength    int64             `json:"cdnFileLength"`
	Digest           string            `json:"digest"`
	SourceRealDigest string            `json:"sourceRealDigest"`
	Tag              string            `json:"tag"`
	ExpireInfo       map[string]string `json:"expireInfo"`
	Finish           bool              `json:"finish"`
	Success          bool              `json:"success"`
	TotalPieceCount  int32             `json:"totalPieceCount"`
	PieceMd5Sign     string            `json:"pieceMd5Sign"`
	Range            string            `json:"range"`
	Filter           string            `json:"filter"`
}

// PieceMetaRecord meta data of piece
type PieceMetaRecord struct {
	// piece Num start from 0
	PieceNum int32 `json:"pieceNum"`
	// 存储到存储介质的真实长度
	PieceLen int32 `json:"pieceLen"`
	// for transported piece content，不是origin source 的 md5，是真是存储到存储介质后的md5（为了读取数据文件时方便校验完整性）
	Md5 string `json:"md5"`
	// 下载存储到磁盘的range，不是origin source的range.提供给客户端发送下载请求,for transported piece content
	Range *rangeutils.Range `json:"range"`
	//  piece's real offset in the file
	OriginRange *rangeutils.Range `json:"originRange"`
	// 1: PlainUnspecified
	PieceStyle base.PieceStyle `json:"pieceStyle"`
}

const fieldSeparator = ":"

func (record PieceMetaRecord) String() string {
	return fmt.Sprint(record.PieceNum, fieldSeparator, record.PieceLen, fieldSeparator, record.Md5, fieldSeparator, record.Range, fieldSeparator,
		record.OriginRange, fieldSeparator, record.PieceStyle)
}

func ParsePieceMetaRecord(value string) (record *PieceMetaRecord, err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = errors.Errorf("%v", msg)
		}
	}()
	fields := strings.Split(value, fieldSeparator)
	pieceNum, err := strconv.ParseInt(fields[0], 10, 32)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid pieceNum: %s", fields[0])
	}
	pieceLen, err := strconv.ParseInt(fields[1], 10, 32)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid pieceLen: %s", fields[1])
	}
	md5 := fields[2]
	pieceRange, err := rangeutils.GetRange(fields[3])
	if err != nil {
		return nil, errors.Wrapf(err, "invalid piece range: %s", fields[3])
	}
	originRange, err := rangeutils.GetRange(fields[4])
	if err != nil {
		return nil, errors.Wrapf(err, "invalid origin range: %s", fields[4])
	}
	pieceStyle, err := strconv.ParseInt(fields[5], 10, 8)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid pieceStyle: %s", fields[5])
	}
	return &PieceMetaRecord{
		PieceNum:    int32(pieceNum),
		PieceLen:    int32(pieceLen),
		Md5:         md5,
		Range:       pieceRange,
		OriginRange: originRange,
		PieceStyle:  base.PieceStyle(pieceStyle),
	}, nil
}

// Builder creates a balancer.
type Builder interface {
	// Build creates a new balancer with the ClientConn.
	Build(cfg Config, taskManager task.Manager) (Manager, error)
	// Name returns the name of balancers built by this builder.
	// It will be used to pick balancers (for example in service config).
	Name() string
}

// Register defines an interface to register a storage manager builder.
// All storage managers should call this function to register its builder to the storage manager factory.
func Register(builder Builder) {
	m[strings.ToLower(builder.Name())] = builder
}

// Get a storage manager from manager with specified name.
func Get(name string) Builder {
	if b, ok := m[strings.ToLower(name)]; ok {
		return b
	}
	return nil
}

type Config struct {
	GCInitialDelay time.Duration            `yaml:"gcInitialDelay"`
	GCInterval     time.Duration            `yaml:"gcInterval"`
	DriverConfigs  map[string]*DriverConfig `yaml:"driverConfigs"`
}

type DriverConfig struct {
	GCConfig *GCConfig `yaml:"gcConfig"`
}

// GCConfig gc config
type GCConfig struct {
	YoungGCThreshold  unit.Bytes    `yaml:"youngGCThreshold"`
	FullGCThreshold   unit.Bytes    `yaml:"fullGCThreshold"`
	CleanRatio        int           `yaml:"cleanRatio"`
	IntervalThreshold time.Duration `yaml:"intervalThreshold"`
}
