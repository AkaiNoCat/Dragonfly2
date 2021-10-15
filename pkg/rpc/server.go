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

package rpc

import (
	"context"
	"time"

	"d7y.io/dragonfly/v2/internal/dferrors"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/rpc/base/common"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

var DefaultServerOptions = []grpc.ServerOption{
	grpc.ConnectionTimeout(10 * time.Second),
	grpc.InitialConnWindowSize(8 * 1024 * 1024),
	grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime: 1 * time.Minute,
	}),
	grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle: 5 * time.Minute,
	}),
	grpc.MaxConcurrentStreams(100),
	grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		streamServerInterceptor,
		grpc_prometheus.StreamServerInterceptor,
		grpc_zap.StreamServerInterceptor(logger.GrpcLogger.Desugar()),
	)),
	grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		unaryServerInterceptor,
		grpc_prometheus.UnaryServerInterceptor,
		grpc_zap.UnaryServerInterceptor(logger.GrpcLogger.Desugar()),
	)),
}

func streamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := handler(srv, ss)
	if err != nil {
		err = convertServerError(err)
		logger.GrpcLogger.Errorf("do stream server error: %v for method: %s", err, info.FullMethod)
	}

	return err
}

func unaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	m, err := handler(ctx, req)
	if err != nil {
		err = convertServerError(err)
		logger.GrpcLogger.Errorf("do unary server error: %v for method: %s", err, info.FullMethod)
	}

	return m, err
}

func convertServerError(err error) error {
	if v, ok := err.(*dferrors.DfError); ok {
		logger.GrpcLogger.Errorf(v.Message)
		if s, e := status.Convert(err).WithDetails(common.NewGrpcDfError(v.Code, v.Message)); e == nil {
			err = s.Err()
		}
	}
	return err
}
