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

package client

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/internal/dfnet"
	"d7y.io/dragonfly/v2/pkg/rpc"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem"
)

func GetClientByAddr(addrs []dfnet.NetAddr, opts ...grpc.DialOption) (CdnClient, error) {
	if len(addrs) == 0 {
		return nil, errors.New("address list of cdn is empty")
	}
	cc := &cdnClient{
		rpc.NewConnection(context.Background(), "cdn", addrs, []rpc.ConnOption{
			rpc.WithConnExpireTime(60 * time.Second),
			rpc.WithDialOption(opts),
		}),
	}
	return cc, nil
}

var once sync.Once
var elasticCdnClient *cdnClient

func GetElasticClientByAddrs(addrs []dfnet.NetAddr, opts ...grpc.DialOption) (CdnClient, error) {
	once.Do(func() {
		elasticCdnClient = &cdnClient{
			rpc.NewConnection(context.Background(), "cdn-elastic", make([]dfnet.NetAddr, 0), []rpc.ConnOption{
				rpc.WithConnExpireTime(30 * time.Second),
				rpc.WithDialOption(opts),
			}),
		}
	})
	err := elasticCdnClient.Connection.AddServerNodes(addrs)
	if err != nil {
		return nil, err
	}
	return elasticCdnClient, nil
}

// CdnClient see cdnsystem.CdnClient
type CdnClient interface {
	ObtainSeeds(ctx context.Context, sr *cdnsystem.SeedRequest, opts ...grpc.CallOption) (*PieceSeedStream, error)

	GetPieceTasks(ctx context.Context, addr dfnet.NetAddr, req *base.PieceTaskRequest, opts ...grpc.CallOption) (*base.PiecePacket, error)

	UpdateState(addrs []dfnet.NetAddr)

	Close() error
}

type cdnClient struct {
	*rpc.Connection
}

var _ CdnClient = (*cdnClient)(nil)

func (cc *cdnClient) getCdnClient(key string, stick bool) (cdnsystem.SeederClient, string, error) {
	clientConn, err := cc.Connection.GetClientConn(key, stick)
	if err != nil {
		return nil, "", errors.Wrapf(err, "get ClientConn for hashKey %s", key)
	}
	return cdnsystem.NewSeederClient(clientConn), clientConn.Target(), nil
}

func (cc *cdnClient) getSeederClientWithTarget(target string) (cdnsystem.SeederClient, error) {
	conn, err := cc.Connection.GetClientConnByTarget(target)
	if err != nil {
		return nil, err
	}
	return cdnsystem.NewSeederClient(conn), nil
}

func (cc *cdnClient) ObtainSeeds(ctx context.Context, sr *cdnsystem.SeedRequest, opts ...grpc.CallOption) (*PieceSeedStream, error) {
	return newPieceSeedStream(ctx, cc, sr.TaskId, sr, opts)
}

func (cc *cdnClient) GetPieceTasks(ctx context.Context, addr dfnet.NetAddr, req *base.PieceTaskRequest, opts ...grpc.CallOption) (*base.PiecePacket, error) {
	res, err := rpc.ExecuteWithRetry(func() (interface{}, error) {
		client, err := cc.getSeederClientWithTarget(addr.GetEndpoint())
		if err != nil {
			return nil, err
		}
		return client.GetPieceTasks(ctx, req, opts...)
	}, 0.2, 2.0, 3, nil)
	if err != nil {
		logger.WithTaskID(req.TaskId).Infof("GetPieceTasks: invoke cdn node %s GetPieceTasks failed: %v", addr.GetEndpoint(), err)
		return nil, err
	}
	return res.(*base.PiecePacket), nil
}
