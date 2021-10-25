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
	"fmt"

	"d7y.io/dragonfly/v2/cdn/cdnutil"
	"d7y.io/dragonfly/v2/cdn/config"
	cdnerrors "d7y.io/dragonfly/v2/cdn/errors"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/internal/dfcodes"
	"d7y.io/dragonfly/v2/internal/dferrors"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/internal/idgen"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem"
	cdnserver "d7y.io/dragonfly/v2/pkg/rpc/cdnsystem/server"
	"d7y.io/dragonfly/v2/pkg/util/net/iputils"
	"d7y.io/dragonfly/v2/pkg/util/net/urlutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

var tracer = otel.Tracer("cdn-server")

type server struct {
	*grpc.Server
	taskMgr supervisor.SeedTaskManager
	cfg     *config.Config
}

// New returns a new Manager Object.
func New(cfg *config.Config, taskMgr supervisor.SeedTaskManager, opts ...grpc.ServerOption) (*grpc.Server, error) {
	svr := &server{
		taskMgr: taskMgr,
		cfg:     cfg,
	}

	svr.Server = cdnserver.New(svr, opts...)
	return svr.Server, nil
}

// checkSeedRequestParams check the params of SeedRequest.
func checkSeedRequestParams(req *cdnsystem.SeedRequest) error {
	if stringutils.IsBlank(req.TaskId) {
		return errors.New("taskId is empty")
	}
	if !urlutils.IsValidURL(req.Url) {
		return errors.Errorf("resource url: %s is invalid", req.Url)
	}
	return nil
}

func (css *server) ObtainSeeds(ctx context.Context, req *cdnsystem.SeedRequest, psc chan<- *cdnsystem.PieceSeed) (err error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanObtainSeeds, trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()
	span.SetAttributes(config.AttributeObtainSeedsRequest.String(req.String()))
	span.SetAttributes(config.AttributeTaskID.String(req.TaskId))
	logger.Infof("obtain seeds request: %+v", req)
	defer func() {
		if r := recover(); r != nil {
			err = dferrors.Newf(dfcodes.UnknownError, "obtain task(%s) seeds encounter an panic: %v", req.TaskId, r)
			span.RecordError(err)
			logger.WithTaskID(req.TaskId).Errorf("%v", err)
		}
		logger.Infof("seeds task %s result success: %t", req.TaskId, err == nil)
	}()
	if err := checkSeedRequestParams(req); err != nil {
		err = dferrors.Newf(dfcodes.BadRequest, "bad seed request for task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return err
	}
	registerTask := types.NewSeedTask(req.TaskId, req.Url, req.UrlMeta)
	// todo
	pieceChan, err := css.seedTask.register()
	if err != nil {

	}
	// register task
	pieceChan, err := css.taskMgr.Register(ctx, registerTask)

	if err != nil {
		if cdnerrors.IsResourcesLacked(err) {
			err = dferrors.Newf(dfcodes.ResourceLacked, "resources lacked for task(%s): %v", req.TaskId, err)
			span.RecordError(err)
			return err
		}
		err = dferrors.Newf(dfcodes.CdnTaskRegistryFail, "failed to register seed task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return err
	}
	peerID := cdnutil.GenCDNPeerID(req.TaskId)
	for piece := range pieceChan {
		psc <- &cdnsystem.PieceSeed{
			PeerId:   peerID,
			HostUuid: idgen.CDN(iputils.HostName, int32(css.cfg.ListenPort)),
			PieceInfo: &base.PieceInfo{
				PieceNum:    piece.PieceNum,
				RangeStart:  piece.PieceRange.StartIndex,
				RangeSize:   piece.PieceLen,
				PieceMd5:    piece.PieceMd5,
				PieceOffset: piece.OriginRange.StartIndex,
				PieceStyle:  base.PieceStyle(piece.PieceStyle),
			},
			Done: false,
		}
	}
	task, err := css.taskMgr.Get(req.TaskId)
	if err != nil {
		err = dferrors.Newf(dfcodes.CdnError, "failed to get task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return err
	}
	if !task.IsSuccess() {
		err = dferrors.Newf(dfcodes.CdnTaskDownloadFail, "task(%s) status error , status: %s", req.TaskId, task.CdnStatus)
		span.RecordError(err)
		return err
	}
	psc <- &cdnsystem.PieceSeed{
		PeerId:          peerID,
		HostUuid:        idgen.CDN(iputils.HostName, int32(css.cfg.ListenPort)),
		Done:            true,
		ContentLength:   task.SourceFileLength,
		TotalPieceCount: task.TotalPieceCount,
	}
	return nil
}

func (css *server) GetPieceTasks(ctx context.Context, req *base.PieceTaskRequest) (piecePacket *base.PiecePacket, err error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanGetPieceTasks, trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()
	span.SetAttributes(config.AttributeGetPieceTasksRequest.String(req.String()))
	span.SetAttributes(config.AttributeTaskID.String(req.TaskId))
	logger.Infof("get piece tasks: %+v", req)
	defer func() {
		if r := recover(); r != nil {
			err = dferrors.Newf(dfcodes.UnknownError, "get task(%s) piece tasks encounter an panic: %v", req.TaskId, r)
			span.RecordError(err)
			logger.WithTaskID(req.TaskId).Errorf("get piece tasks failed: %v", err)
		}
		logger.WithTaskID(req.TaskId).Infof("get piece tasks result success: %t", err == nil)
	}()
	if err := checkPieceTasksRequestParams(req); err != nil {
		err = dferrors.Newf(dfcodes.BadRequest, "bad request for task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return nil, err
	}
	task, err := css.taskMgr.Get(req.TaskId)
	if err != nil {
		if cdnerrors.IsDataNotFound(err) {
			err = dferrors.Newf(dfcodes.CdnTaskNotFound, "failed to get task(%s) from cdn: %v", req.TaskId, err)
			span.RecordError(err)
			return nil, err
		}
		err = dferrors.Newf(dfcodes.CdnError, "failed to get task(%s) from cdn: %v", req.TaskId, err)
		span.RecordError(err)
		return nil, err
	}
	if task.IsError() {
		err = dferrors.Newf(dfcodes.CdnTaskDownloadFail, "task(%s) status is FAIL, cdnStatus: %s", task.ID, task.CdnStatus)
		span.RecordError(err)
		return nil, err
	}
	pieces, err := css.taskMgr.GetPieces(ctx, req.TaskId)
	if err != nil {
		err = dferrors.Newf(dfcodes.CdnError, "failed to get pieces of task(%s) from cdn: %v", task.ID, err)
		span.RecordError(err)
		return nil, err
	}
	pieceInfos := make([]*base.PieceInfo, 0)
	var count int32 = 0
	for _, piece := range pieces {
		if piece.PieceNum >= req.StartNum && (count < req.Limit || req.Limit == 0) {
			p := &base.PieceInfo{
				PieceNum:    piece.PieceNum,
				RangeStart:  piece.PieceRange.StartIndex,
				RangeSize:   piece.PieceLen,
				PieceMd5:    piece.PieceMd5,
				PieceOffset: piece.OriginRange.StartIndex,
				PieceStyle:  base.PieceStyle(piece.PieceStyle),
			}
			pieceInfos = append(pieceInfos, p)
			count++
		}
	}
	pp := &base.PiecePacket{
		TaskId:        req.TaskId,
		DstPid:        fmt.Sprintf("%s-%s_%s", iputils.HostName, req.TaskId, "CDN"),
		DstAddr:       fmt.Sprintf("%s:%d", css.cfg.AdvertiseIP, css.cfg.DownloadPort),
		PieceInfos:    pieceInfos,
		TotalPiece:    task.TotalPieceCount,
		ContentLength: task.SourceFileLength,
		PieceMd5Sign:  task.PieceMd5Sign,
	}
	span.SetAttributes(config.AttributePiecePacketResult.String(pp.String()))
	return pp, nil
}

func checkPieceTasksRequestParams(req *base.PieceTaskRequest) error {
	if stringutils.IsBlank(req.TaskId) {
		return errors.Errorf("taskID is empty")
	}
	if stringutils.IsBlank(req.SrcPid) {
		return errors.Errorf("src peerID is empty")
	}
	if req.StartNum < 0 {
		return errors.Errorf("invalid starNum %d", req.StartNum)
	}
	if req.Limit < 0 {
		return errors.Errorf("invalid limit %d", req.Limit)
	}
	return nil
}
