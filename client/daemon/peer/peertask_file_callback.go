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
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"

	"d7y.io/dragonfly/v2/client/config"
	"d7y.io/dragonfly/v2/client/daemon/storage"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/scheduler"
)

type filePeerTaskCallback struct {
	ptm   *peerTaskManager
	pt    *filePeerTask
	req   *FilePeerTaskRequest
	start time.Time
}

var _ TaskCallback = (*filePeerTaskCallback)(nil)

func (p *filePeerTaskCallback) GetStartTime() time.Time {
	return p.start
}

func (p *filePeerTaskCallback) Init(pt Task) error {
	// prepare storage
	err := p.ptm.storageManager.RegisterTask(p.pt.ctx,
		storage.RegisterTaskRequest{
			CommonTaskRequest: storage.CommonTaskRequest{
				PeerID:      pt.GetPeerID(),
				TaskID:      pt.GetTaskID(),
				Destination: p.req.Output,
			},
			ContentLength: pt.GetContentLength(),
			TotalPieces:   pt.GetTotalPieces(),
			PieceMd5Sign:  pt.GetPieceMd5Sign(),
		})
	if err != nil {
		pt.Log().Errorf("register task to storage manager failed: %s", err)
	}
	return err
}

func (p *filePeerTaskCallback) Update(pt Task) error {
	// update storage
	err := p.ptm.storageManager.UpdateTask(p.pt.ctx,
		&storage.UpdateTaskRequest{
			PeerTaskMetadata: storage.PeerTaskMetadata{
				PeerID: pt.GetPeerID(),
				TaskID: pt.GetTaskID(),
			},
			ContentLength: pt.GetContentLength(),
			TotalPieces:   pt.GetTotalPieces(),
			PieceMd5Sign:  pt.GetPieceMd5Sign(),
		})
	if err != nil {
		pt.Log().Errorf("update task to storage manager failed: %s", err)
	}
	return err
}

func (p *filePeerTaskCallback) Done(pt Task) error {
	var cost = time.Now().Sub(p.start).Milliseconds()
	pt.Log().Infof("file peer task done, cost: %dms", cost)
	e := p.ptm.storageManager.Store(
		p.pt.ctx,
		&storage.StoreRequest{
			CommonTaskRequest: storage.CommonTaskRequest{
				PeerID:      pt.GetPeerID(),
				TaskID:      pt.GetTaskID(),
				Destination: p.req.Output,
			},
			MetadataOnly: false,
			TotalPieces:  pt.GetTotalPieces(),
		})
	if e != nil {
		return e
	}
	p.ptm.PeerTaskDone(p.req.PeerId)
	ctx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(p.pt.ctx))
	peerResultCtx, peerResultSpan := tracer.Start(ctx, config.SpanReportPeerResult)
	defer peerResultSpan.End()
	err := p.pt.schedulerClient.ReportPeerResult(peerResultCtx, &scheduler.PeerResult{
		TaskId:          pt.GetTaskID(),
		PeerId:          pt.GetPeerID(),
		SrcIp:           p.ptm.host.Ip,
		SecurityDomain:  p.ptm.host.SecurityDomain,
		Idc:             p.ptm.host.Idc,
		Url:             p.req.Url,
		ContentLength:   pt.GetContentLength(),
		Traffic:         pt.GetTraffic(),
		TotalPieceCount: pt.GetTotalPieces(),
		Cost:            uint32(cost),
		Success:         true,
		Code:            base.Code_Success,
	})
	if err != nil {
		peerResultSpan.RecordError(err)
		pt.Log().Errorf("step 3: report successful peer result, error: %v", err)
	} else {
		pt.Log().Infof("step 3: report successful peer result ok")
	}
	return nil
}

func (p *filePeerTaskCallback) Fail(pt Task, code base.Code, reason string) error {
	p.ptm.PeerTaskDone(p.req.PeerId)
	var end = time.Now()
	pt.Log().Errorf("file peer task failed, code: %d, reason: %s", code, reason)
	ctx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(p.pt.ctx))
	peerResultCtx, peerResultSpan := tracer.Start(ctx, config.SpanReportPeerResult)
	defer peerResultSpan.End()
	err := p.pt.schedulerClient.ReportPeerResult(peerResultCtx, &scheduler.PeerResult{
		TaskId:          pt.GetTaskID(),
		PeerId:          pt.GetPeerID(),
		SrcIp:           p.ptm.host.Ip,
		SecurityDomain:  p.ptm.host.SecurityDomain,
		Idc:             p.ptm.host.Idc,
		Url:             p.req.Url,
		ContentLength:   pt.GetContentLength(),
		Traffic:         pt.GetTraffic(),
		TotalPieceCount: p.pt.totalPiece,
		Cost:            uint32(end.Sub(p.start).Milliseconds()),
		Success:         false,
		Code:            code,
	})
	if err != nil {
		peerResultSpan.RecordError(err)
		pt.Log().Errorf("step 3: report fail peer result, error: %v", err)
	} else {
		pt.Log().Infof("step 3: report fail peer result ok")
	}
	return nil
}

func (p *filePeerTaskCallback) ValidateDigest(pt Task) error {
	if !p.ptm.calculateDigest {
		return nil
	}
	err := p.ptm.storageManager.ValidateDigest(
		&storage.PeerTaskMetadata{
			PeerID: pt.GetPeerID(),
			TaskID: pt.GetTaskID(),
		})
	if err != nil {
		pt.Log().Errorf("%s", err)
	} else {
		pt.Log().Debugf("validated digest")
	}
	return err
}
