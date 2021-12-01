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

package peer

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	testifyassert "github.com/stretchr/testify/assert"
	testifyrequire "github.com/stretchr/testify/require"

	"d7y.io/dragonfly/v2/cdn/cdnutil"
	"d7y.io/dragonfly/v2/client/clientutil"
	"d7y.io/dragonfly/v2/client/config"
	"d7y.io/dragonfly/v2/client/daemon/test"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/scheduler"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/source/httpprotocol"
	sourceMock "d7y.io/dragonfly/v2/pkg/source/mock"
)

func TestStreamPeerTask_BackSource_WithContentLength(t *testing.T) {
	assert := testifyassert.New(t)
	require := testifyrequire.New(t)
	ctrl := gomock.NewController(t)

	testBytes, err := ioutil.ReadFile(test.File)
	assert.Nil(err, "load test file")

	var (
		pieceParallelCount = int32(4)
		pieceSize          = 1024

		mockContentLength = len(testBytes)
		//mockPieceCount    = int(math.Ceil(float64(mockContentLength) / float64(pieceSize)))

		peerID = "peer-back-source-out-content-length"
		taskID = "task-back-source-out-content-length"

		url = "http://localhost/test/data"
	)
	schedulerClient, storageManager := setupPeerTaskManagerComponents(
		ctrl,
		componentsOption{
			taskID:             taskID,
			contentLength:      int64(mockContentLength),
			pieceSize:          int32(pieceSize),
			pieceParallelCount: pieceParallelCount,
		})
	defer storageManager.CleanUp()

	downloader := NewMockPieceDownloader(ctrl)
	downloader.EXPECT().DownloadPiece(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, task *DownloadPieceRequest) (io.Reader, io.Closer, error) {
			rc := ioutil.NopCloser(
				bytes.NewBuffer(
					testBytes[task.piece.RangeStart : task.piece.RangeStart+uint64(task.piece.RangeSize)],
				))
			return rc, rc, nil
		})

	request, err := source.NewRequest(url)
	assert.Nil(err, "create request")
	sourceClient := sourceMock.NewMockResourceClient(ctrl)
	source.UnRegister("http")
	require.Nil(source.Register("http", sourceClient, httpprotocol.Adapter))
	defer source.UnRegister("http")
	sourceClient.EXPECT().GetContentLength(source.RequestEq(request.URL.String())).DoAndReturn(
		func(request *source.Request) (int64, error) {
			return int64(len(testBytes)), nil
		})
	sourceClient.EXPECT().Download(source.RequestEq(request.URL.String())).DoAndReturn(
		func(request *source.Request) (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewBuffer(testBytes)), nil
		})

	ptm := &peerTaskManager{
		host: &scheduler.PeerHost{
			Ip: "127.0.0.1",
		},
		runningPeerTasks: sync.Map{},
		pieceManager: &pieceManager{
			storageManager:   storageManager,
			pieceDownloader:  downloader,
			computePieceSize: cdnutil.ComputePieceSize,
		},
		storageManager:  storageManager,
		schedulerClient: schedulerClient,
		schedulerOption: config.SchedulerOption{
			ScheduleTimeout: clientutil.Duration{Duration: 10 * time.Minute},
		},
	}
	req := &scheduler.PeerTaskRequest{
		Url: url,
		UrlMeta: &base.UrlMeta{
			Tag: "d7y-test",
		},
		PeerId:   peerID,
		PeerHost: &scheduler.PeerHost{},
	}
	ctx := context.Background()
	_, pt, _, err := newStreamPeerTask(ctx,
		ptm.host,
		&pieceManager{
			storageManager:  storageManager,
			pieceDownloader: downloader,
			computePieceSize: func(contentLength int64) int32 {
				return int32(pieceSize)
			},
		},
		req,
		ptm.schedulerClient,
		ptm.schedulerOption,
		0)
	assert.Nil(err, "new stream peer task")
	pt.SetCallback(&streamPeerTaskCallback{
		ptm:   ptm,
		pt:    pt,
		req:   req,
		start: time.Now(),
	})
	pt.needBackSource = true

	rc, _, err := pt.Start(ctx)
	assert.Nil(err, "start stream peer task")

	outputBytes, err := ioutil.ReadAll(rc)
	assert.Nil(err, "load read data")
	assert.Equal(testBytes, outputBytes, "output and desired output must match")
}

func TestStreamPeerTask_BackSource_WithoutContentLength(t *testing.T) {
	assert := testifyassert.New(t)
	require := testifyrequire.New(t)
	ctrl := gomock.NewController(t)

	testBytes, err := ioutil.ReadFile(test.File)
	assert.Nil(err, "load test file")

	var (
		pieceParallelCount = int32(4)
		pieceSize          = 1024

		mockContentLength = len(testBytes)
		//mockPieceCount    = int(math.Ceil(float64(mockContentLength) / float64(pieceSize)))

		peerID = "peer-back-source-without-content-length"
		taskID = "task-back-source-without-content-length"

		url = "http://localhost/test/data"
	)
	schedulerClient, storageManager := setupPeerTaskManagerComponents(
		ctrl,
		componentsOption{
			taskID:             taskID,
			contentLength:      int64(mockContentLength),
			pieceSize:          int32(pieceSize),
			pieceParallelCount: pieceParallelCount,
		})
	defer storageManager.CleanUp()

	downloader := NewMockPieceDownloader(ctrl)
	downloader.EXPECT().DownloadPiece(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, task *DownloadPieceRequest) (io.Reader, io.Closer, error) {
			rc := ioutil.NopCloser(
				bytes.NewBuffer(
					testBytes[task.piece.RangeStart : task.piece.RangeStart+uint64(task.piece.RangeSize)],
				))
			return rc, rc, nil
		})

	sourceClient := sourceMock.NewMockResourceClient(ctrl)
	source.UnRegister("http")
	require.Nil(source.Register("http", sourceClient, httpprotocol.Adapter))
	defer source.UnRegister("http")
	request, err := source.NewRequest(url)
	assert.Nil(err, "create reqeust")
	sourceClient.EXPECT().GetContentLength(source.RequestEq(request.URL.String())).DoAndReturn(
		func(request *source.Request) (int64, error) {
			return -1, nil
		})
	sourceClient.EXPECT().Download(source.RequestEq(request.URL.String())).DoAndReturn(
		func(request *source.Request) (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewBuffer(testBytes)), nil
		})

	ptm := &peerTaskManager{
		host: &scheduler.PeerHost{
			Ip: "127.0.0.1",
		},
		runningPeerTasks: sync.Map{},
		pieceManager: &pieceManager{
			storageManager:   storageManager,
			pieceDownloader:  downloader,
			computePieceSize: cdnutil.ComputePieceSize,
		},
		storageManager:  storageManager,
		schedulerClient: schedulerClient,
		schedulerOption: config.SchedulerOption{
			ScheduleTimeout: clientutil.Duration{Duration: 10 * time.Minute},
		},
	}
	req := &scheduler.PeerTaskRequest{
		Url: url,
		UrlMeta: &base.UrlMeta{
			Tag: "d7y-test",
		},
		PeerId:   peerID,
		PeerHost: &scheduler.PeerHost{},
	}
	ctx := context.Background()
	_, pt, _, err := newStreamPeerTask(ctx,
		ptm.host,
		&pieceManager{
			storageManager:  storageManager,
			pieceDownloader: downloader,
			computePieceSize: func(contentLength int64) int32 {
				return int32(pieceSize)
			},
		},
		req,
		schedulerClient,
		ptm.schedulerOption,
		0)
	assert.Nil(err, "new stream peer task")
	pt.SetCallback(&streamPeerTaskCallback{
		ptm:   ptm,
		pt:    pt,
		req:   req,
		start: time.Now(),
	})
	pt.needBackSource = true

	rc, _, err := pt.Start(ctx)
	assert.Nil(err, "start stream peer task")

	outputBytes, err := ioutil.ReadAll(rc)
	assert.Nil(err, "load read data")
	assert.Equal(testBytes, outputBytes, "output and desired output must match")
}
