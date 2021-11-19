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
	"context"
	"crypto/md5"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"

	"d7y.io/dragonfly/v2/cdn/storedriver"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	storageMock "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/mock"
	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/pkg/source"
	sourceMock "d7y.io/dragonfly/v2/pkg/source/mock"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"d7y.io/dragonfly/v2/pkg/util/rangeutils"
)

func TestCacheDetectorSuite(t *testing.T) {
	suite.Run(t, new(CacheDetectorTestSuite))
}

type CacheDetectorTestSuite struct {
	detector *cacheDetector
	suite.Suite
}

func (suite *CacheDetectorTestSuite) SetupSuite() {
	ctrl := gomock.NewController(suite.T())
	sourceClient := sourceMock.NewMockResourceClient(ctrl)
	source.Register("http", sourceClient, func(request *source.Request) *source.Request {
		return request
	})
	storageMgr := storageMock.NewMockManager(ctrl)
	cacheDataManager := newMetadataManager(storageMgr)
	suite.detector = newCacheDetector(cacheDataManager)
	storageMgr.EXPECT().ReadFileMetadata(fullExpiredCache.taskID).Return(fullExpiredCache.fileMeta, nil).AnyTimes()
	storageMgr.EXPECT().ReadFileMetadata(fullNoExpiredCache.taskID).Return(fullNoExpiredCache.fileMeta, nil).AnyTimes()
	storageMgr.EXPECT().ReadFileMetadata(partialNotSupportRangeCache.taskID).Return(partialNotSupportRangeCache.fileMeta, nil).AnyTimes()
	storageMgr.EXPECT().ReadFileMetadata(partialSupportRangeCache.taskID).Return(partialSupportRangeCache.fileMeta, nil).AnyTimes()
	storageMgr.EXPECT().ReadFileMetadata(noCache.taskID).Return(noCache.fileMeta, os.ErrNotExist).AnyTimes()
	storageMgr.EXPECT().ReadDownloadFile(fullNoExpiredCache.taskID).DoAndReturn(
		func(taskID string) (io.ReadCloser, error) {
			content, err := ioutil.ReadFile("../../testdata/cdn/go.html")
			suite.Nil(err)
			return ioutil.NopCloser(strings.NewReader(string(content))), nil
		}).AnyTimes()
	storageMgr.EXPECT().ReadDownloadFile(partialNotSupportRangeCache.taskID).DoAndReturn(
		func(taskID string) (io.ReadCloser, error) {
			content, err := ioutil.ReadFile("../../testdata/cdn/go.html")
			suite.Nil(err)
			return ioutil.NopCloser(strings.NewReader(string(content))), nil
		}).AnyTimes()
	storageMgr.EXPECT().ReadDownloadFile(partialSupportRangeCache.taskID).DoAndReturn(
		func(taskID string) (io.ReadCloser, error) {
			content, err := ioutil.ReadFile("../../testdata/cdn/go.html")
			suite.Nil(err)
			return ioutil.NopCloser(strings.NewReader(string(content))), nil
		}).AnyTimes()
	storageMgr.EXPECT().ReadDownloadFile(noCache.taskID).Return(nil, os.ErrNotExist).AnyTimes()
	storageMgr.EXPECT().ReadPieceMetaRecords(fullNoExpiredCache.taskID).Return(fullNoExpiredCache.pieces, nil).AnyTimes()
	storageMgr.EXPECT().ReadPieceMetaRecords(partialNotSupportRangeCache.taskID).Return(partialNotSupportRangeCache.pieces, nil).AnyTimes()
	storageMgr.EXPECT().ReadPieceMetaRecords(partialSupportRangeCache.taskID).Return(partialSupportRangeCache.pieces, nil).AnyTimes()
	storageMgr.EXPECT().ReadPieceMetaRecords(noCache.taskID).Return(nil, os.ErrNotExist).AnyTimes()
	storageMgr.EXPECT().StatDownloadFile(fullNoExpiredCache.taskID).Return(&storedriver.StorageInfo{
		Path:       "",
		Size:       9789,
		CreateTime: time.Time{},
		ModTime:    time.Time{},
	}, nil).AnyTimes()

	sourceClient.EXPECT().IsExpired(gomock.Eq(expiredAndSupportRequest), gomock.Any()).Return(true, nil).AnyTimes()
	sourceClient.EXPECT().IsSupportRange(gomock.Eq(expiredAndSupportRequest)).Return(true, nil).AnyTimes()

	sourceClient.EXPECT().IsExpired(gomock.Eq(noExpiredAndNotSupportRequest), gomock.Any()).Return(true, nil).AnyTimes()
	sourceClient.EXPECT().IsSupportRange(gomock.Eq(noExpiredAndNotSupportRequest)).Return(false, nil).AnyTimes()

	sourceClient.EXPECT().IsExpired(gomock.Eq(noExpiredAndSupportRequest), gomock.Any()).Return(false, nil).AnyTimes()
	sourceClient.EXPECT().IsSupportRange(gomock.Eq(noExpiredAndSupportRequest)).Return(true, nil).AnyTimes()

	sourceClient.EXPECT().IsExpired(gomock.Eq(noExpiredAndNotSupportRequest), gomock.Any()).Return(false, nil).AnyTimes()
	sourceClient.EXPECT().IsSupportRange(gomock.Eq(noExpiredAndNotSupportRequest)).Return(false, nil).AnyTimes()
}

var noCacheTask, partialAndSupportCacheTask, partialAndNotSupportCacheTask, fullCacheExpiredTask, fullCacheNotExpiredTask = "noCache", "partialSupportCache",
	"partialNotSupportCache", "fullCache", "fullCacheNotExpired"

var expiredAndSupportURL, expiredAndNotSupportURL, noExpiredAndSupportURL, noExpiredAndNotSupportURL = "http://expiredsupport.com",
	"http://expiredNotsupport.com", "http://noexpiredAndsupport.com", "http://noexpiredAndnotsupport.com"

var expiredAndSupportRequest, _ = source.NewRequest(expiredAndSupportURL)

var expiredAndNotSupporRequest, _ = source.NewRequest(expiredAndNotSupportURL)

var noExpiredAndSupportRequest, _ = source.NewRequest(noExpiredAndSupportURL)

var noExpiredAndNotSupportRequest, _ = source.NewRequest(noExpiredAndNotSupportURL)

type mockData struct {
	taskID   string
	pieces   []*storage.PieceMetaRecord
	fileMeta *storage.FileMetadata
	reader   io.ReadCloser
}

var fullNoExpiredCache = mockData{
	taskID:   fullCacheNotExpiredTask,
	pieces:   fullPieceMetaRecords,
	fileMeta: newCompletedFileMeta(fullCacheNotExpiredTask, noExpiredAndNotSupportURL, true),
}

var fullExpiredCache = mockData{
	taskID:   fullCacheExpiredTask,
	pieces:   fullPieceMetaRecords,
	fileMeta: newCompletedFileMeta(fullCacheExpiredTask, noExpiredAndSupportURL, true),
}

var partialSupportRangeCache = mockData{
	taskID:   partialAndSupportCacheTask,
	pieces:   partialPieceMetaRecords,
	fileMeta: newPartialFileMeta(partialAndSupportCacheTask, noExpiredAndSupportURL),
}

var partialNotSupportRangeCache = mockData{
	taskID:   partialAndNotSupportCacheTask,
	pieces:   partialPieceMetaRecords,
	fileMeta: newPartialFileMeta(partialAndNotSupportCacheTask, noExpiredAndNotSupportURL),
}

var noCache = mockData{
	taskID:   noCacheTask,
	pieces:   nil,
	fileMeta: nil,
}

var partialPieceMetaRecords = []*storage.PieceMetaRecord{
	{
		PieceNum: 1,
		PieceLen: 2000,
		Md5:      "67e186642cc5d1b43713379955af82bd",
		Range: &rangeutils.Range{
			StartIndex: 2000,
			EndIndex:   3999,
		},
		OriginRange: &rangeutils.Range{
			StartIndex: 2000,
			EndIndex:   3999,
		},
		PieceStyle: 1,
	}, {
		PieceNum: 0,
		PieceLen: 2000,
		Md5:      "4a6cf46821d4fb237bc2179bb5bedfa6",
		Range: &rangeutils.Range{
			StartIndex: 0,
			EndIndex:   1999,
		},
		OriginRange: &rangeutils.Range{
			StartIndex: 0,
			EndIndex:   1999,
		},
		PieceStyle: 1,
	},
}
var fullPieceMetaRecords = append(partialPieceMetaRecords, &storage.PieceMetaRecord{
	PieceNum: 2,
	PieceLen: 2000,
	Md5:      "5ca91ba695d24ad36f25ea350750c9fe",
	Range: &rangeutils.Range{
		StartIndex: 4000,
		EndIndex:   5999,
	},
	OriginRange: &rangeutils.Range{
		StartIndex: 4000,
		EndIndex:   5999,
	},
	PieceStyle: 1,
}, &storage.PieceMetaRecord{
	PieceNum: 3,
	PieceLen: 2000,
	Md5:      "0408118a35af5084043eabcea19c8695",
	Range: &rangeutils.Range{
		StartIndex: 6000,
		EndIndex:   7999,
	},
	OriginRange: &rangeutils.Range{
		StartIndex: 6000,
		EndIndex:   7999,
	},
	PieceStyle: 1,
}, &storage.PieceMetaRecord{
	PieceNum: 4,
	PieceLen: 1789,
	Md5:      "04de99cd9b578ff0e4a8ed7f382316e0",
	Range: &rangeutils.Range{
		StartIndex: 8000,
		EndIndex:   9788,
	},
	OriginRange: &rangeutils.Range{
		StartIndex: 8000,
		EndIndex:   9788,
	},
	PieceStyle: 1,
})

func newCompletedFileMeta(taskID string, URL string, success bool) *storage.FileMetadata {
	return &storage.FileMetadata{
		TaskID:           taskID,
		TaskURL:          URL,
		PieceSize:        2000,
		SourceFileLen:    9789,
		AccessTime:       1624126443284,
		Interval:         0,
		CdnFileLength:    9789,
		SourceRealDigest: "",
		PieceMd5Sign:     "98166bdfebb7b71dd5c6d47492d844f4421d90199641ca11fd8ce3111894115a",
		ExpireInfo:       nil,
		Finish:           true,
		Success:          success,
		TotalPieceCount:  5,
	}
}

func newPartialFileMeta(taskID string, URL string) *storage.FileMetadata {
	return &storage.FileMetadata{
		TaskID:           taskID,
		TaskURL:          URL,
		PieceSize:        2000,
		SourceFileLen:    9789,
		AccessTime:       1624126443284,
		Interval:         0,
		CdnFileLength:    0,
		SourceRealDigest: "",
		PieceMd5Sign:     "",
		ExpireInfo:       nil,
		Finish:           false,
		Success:          false,
		TotalPieceCount:  0,
	}
}

func (suite *CacheDetectorTestSuite) TestDetectCache() {
	type args struct {
		task *types.SeedTask
	}
	tests := []struct {
		name    string
		args    args
		want    *cacheResult
		wantErr bool
	}{
		{
			name: "no cache",
			args: args{
				task: &types.SeedTask{
					ID:      noCacheTask,
					RawURL:  noExpiredAndSupportURL,
					TaskURL: noExpiredAndSupportURL,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "partial cache and support range",
			args: args{
				task: &types.SeedTask{
					ID:               partialAndSupportCacheTask,
					RawURL:           noExpiredAndSupportURL,
					TaskURL:          noExpiredAndSupportURL,
					SourceFileLength: 9789,
					PieceSize:        2000,
				},
			},
			want: &cacheResult{
				breakPoint:       4000,
				pieceMetaRecords: partialPieceMetaRecords,
				fileMetadata:     newPartialFileMeta(partialAndSupportCacheTask, noExpiredAndSupportURL),
			},
			wantErr: false,
		},
		{
			name: "partial cache and not support range",
			args: args{
				task: &types.SeedTask{
					ID:               partialAndNotSupportCacheTask,
					RawURL:           noExpiredAndNotSupportURL,
					TaskURL:          noExpiredAndNotSupportURL,
					SourceFileLength: 9789,
					PieceSize:        2000,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "full cache and not expire",
			args: args{
				task: &types.SeedTask{
					ID:               fullCacheNotExpiredTask,
					RawURL:           noExpiredAndNotSupportURL,
					TaskURL:          noExpiredAndNotSupportURL,
					SourceFileLength: 9789,
					PieceSize:        2000,
				},
			},
			want: &cacheResult{
				breakPoint:       -1,
				pieceMetaRecords: fullPieceMetaRecords,
				fileMetadata:     newCompletedFileMeta(fullCacheNotExpiredTask, noExpiredAndNotSupportURL, true),
			},
			wantErr: false,
		},
		{
			name: "full cache and expired",
			args: args{
				task: &types.SeedTask{
					ID:      fullCacheExpiredTask,
					RawURL:  expiredAndSupportURL,
					TaskURL: expiredAndNotSupportURL,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			digest := md5.New()
			got, err := suite.detector.doDetect(context.Background(), tt.args.task, digest)
			suite.Equal(tt.want, got)
			suite.Equal(err != nil, tt.wantErr)
		})
	}
}

func (suite *CacheDetectorTestSuite) TestParseByReadFile() {
	type args struct {
		taskID   string
		metadata *storage.FileMetadata
	}
	tests := []struct {
		name    string
		args    args
		want    *cacheResult
		wantErr bool
	}{
		{
			name: "partial And SupportCacheTask",
			args: args{
				taskID:   partialSupportRangeCache.taskID,
				metadata: partialSupportRangeCache.fileMeta,
			},
			want: &cacheResult{
				breakPoint:       4000,
				pieceMetaRecords: partialSupportRangeCache.pieces,
				fileMetadata:     partialSupportRangeCache.fileMeta,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			got, err := suite.detector.detectByReadFile(tt.args.taskID, tt.args.metadata, md5.New())
			suite.Equal(tt.want, got)
			suite.Equal(tt.wantErr, err != nil)
		})
	}
}

func (suite *CacheDetectorTestSuite) TestParseByReadMetaFile() {
	type args struct {
		taskID       string
		fileMetadata *storage.FileMetadata
	}
	tests := []struct {
		name    string
		args    args
		want    *cacheResult
		wantErr bool
	}{
		{
			name: "parse full cache file meta",
			args: args{
				taskID:       fullNoExpiredCache.taskID,
				fileMetadata: fullNoExpiredCache.fileMeta,
			},
			want: &cacheResult{
				breakPoint:       -1,
				pieceMetaRecords: fullNoExpiredCache.pieces,
				fileMetadata:     fullNoExpiredCache.fileMeta,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			got, err := suite.detector.detectByReadMetaFile(tt.args.taskID, tt.args.fileMetadata)
			suite.Equal(tt.wantErr, err != nil)
			suite.Equal(tt.want, got)
		})
	}
}

func (suite *CacheDetectorTestSuite) TestCheckPieceContent() {
	content, err := ioutil.ReadFile("../../testdata/cdn/go.html")
	suite.Nil(err)
	type args struct {
		reader       io.Reader
		pieceRecords []*storage.PieceMetaRecord
		fileMd5      hash.Hash
	}
	tests := []struct {
		name        string
		args        args
		wantFileMd5 string
	}{
		{
			name: "check partial cache piece content",
			args: args{
				reader:       strings.NewReader(string(content)),
				pieceRecords: partialSupportRangeCache.pieces,
				fileMd5:      md5.New(),
			},
			wantFileMd5: "ddff04669628a76b52d32322e24a9dd8",
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// sort piece meta records by pieceNum
			sort.Slice(tt.args.pieceRecords, func(i, j int) bool {
				return tt.args.pieceRecords[i].PieceNum < tt.args.pieceRecords[j].PieceNum
			})
			for _, pieceRecord := range tt.args.pieceRecords {
				err := checkPieceContent(tt.args.reader, pieceRecord, tt.args.fileMd5)
				suite.Nil(err)
			}
			suite.Equal(tt.wantFileMd5, digestutils.ToHashString(tt.args.fileMd5))
		})
	}
}
