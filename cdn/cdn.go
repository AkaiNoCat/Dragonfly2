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

	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/gc"
	"d7y.io/dragonfly/v2/cdn/httpserver"
	"d7y.io/dragonfly/v2/cdn/rpcserver"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	"d7y.io/dragonfly/v2/cdn/supervisor/progress"
	"d7y.io/dragonfly/v2/cdn/supervisor/proxy"
	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/rpc/manager"
	managerClient "d7y.io/dragonfly/v2/pkg/rpc/manager/client"
	"d7y.io/dragonfly/v2/pkg/util/hostutils"
)

type Server struct {
	// Server configuration
	config *config.Config

	// GRPC server
	grpcServer *rpcserver.Server

	// HTTP server
	httpServer *httpserver.Server

	// Manager client
	configServer managerClient.Client

	// gc Server
	gcServer *gc.Server
}

// New creates a brand-new server instance.
func New(config *config.Config) (*Server, error) {
	// Initialize task manager
	taskManager, err := task.NewManager(config.Task)
	if err != nil {
		return nil, errors.Wrapf(err, "create task manager")
	}

	// Initialize progress manager
	progressManager, err := progress.NewManager(taskManager)
	if err != nil {
		return nil, errors.Wrapf(err, "create progress manager")
	}

	// Initialize storage manager
	storageManager, err := storage.NewManager(config.Storage, taskManager)
	if err != nil {
		return nil, errors.Wrapf(err, "create storage manager")
	}

	// Initialize proxy manager
	proxyManager, err := proxy.NewManager(config.Proxy)
	if err != nil {
		return nil, errors.Wrapf(err, "create proxy manager")
	}
	// Initialize CDN manager
	cdnManager, err := cdn.NewManager(config.CDN, storageManager, progressManager, taskManager, proxyManager)
	if err != nil {
		return nil, errors.Wrapf(err, "create cdn manager")
	}

	// Initialize CDN service
	service, err := supervisor.NewCDNService(taskManager, cdnManager, progressManager)
	if err != nil {
		return nil, errors.Wrapf(err, "create cdn service")
	}
	// Initialize storage manager
	var opts []grpc.ServerOption
	if config.Options.Telemetry.Jaeger != "" {
		opts = append(opts, grpc.ChainUnaryInterceptor(otelgrpc.UnaryServerInterceptor()), grpc.ChainStreamInterceptor(otelgrpc.StreamServerInterceptor()))
	}
	grpcServer, err := rpcserver.New(config.RPCServer, service, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "create rpcServer")
	}

	// Initialize gc server
	gcServer, err := gc.New()
	if err != nil {
		return nil, errors.Wrap(err, "create gcServer")
	}

	var httpServer *httpserver.Server
	if config.HTTPServer.Addr != "" {
		// Initialize metrics server
		httpServer, err = httpserver.New(config.HTTPServer, grpcServer.Server)
		if err != nil {
			return nil, errors.Wrap(err, "create metricsServer")
		}
	}

	// Initialize configServer
	var configServer managerClient.Client
	if config.Manager.Addr != "" {
		configServer, err = managerClient.New(config.Manager.Addr)
		if err != nil {
			return nil, errors.Wrap(err, "create configServer")
		}
	}
	return &Server{
		config:       config,
		grpcServer:   grpcServer,
		httpServer:   httpServer,
		configServer: configServer,
		gcServer:     gcServer,
	}, nil
}

func (s *Server) Serve() error {
	go func() {
		// Start GC
		if err := s.gcServer.Serve(); err != nil {
			logger.Fatalf("start gc task failed: %v", err)
		}
	}()

	go func() {
		if s.httpServer != nil {
			// Start metrics server
			if err := s.httpServer.ListenAndServe(); err != nil {
				logger.Fatalf("start metrics server failed: %v", err)
			}
		}
	}()

	go func() {
		if s.configServer != nil {
			var rpcServerConfig = s.grpcServer.GetConfig()
			CDNInstance, err := s.configServer.UpdateCDN(&manager.UpdateCDNRequest{
				SourceType:   manager.SourceType_CDN_SOURCE,
				HostName:     hostutils.FQDNHostname,
				Ip:           rpcServerConfig.AdvertiseIP,
				Port:         int32(rpcServerConfig.ListenPort),
				DownloadPort: int32(rpcServerConfig.DownloadPort),
				Idc:          s.config.Host.IDC,
				Location:     s.config.Host.Location,
				CdnClusterId: uint64(s.config.Manager.CDNClusterID),
			})
			if err != nil {
				logger.Fatalf("update cdn instance failed: %v", err)
			}
			// Serve Keepalive
			logger.Infof("====starting keepalive cdn instance %s to manager %s====", CDNInstance, s.config.Manager.Addr)
			s.configServer.KeepAlive(s.config.Manager.KeepAlive.Interval, &manager.KeepAliveRequest{
				HostName:   hostutils.FQDNHostname,
				SourceType: manager.SourceType_CDN_SOURCE,
				ClusterId:  uint64(s.config.Manager.CDNClusterID),
			})
		}
	}()

	// Start grpc server
	return s.grpcServer.ListenAndServe()
}

func (s *Server) Stop() error {
	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return s.gcServer.Shutdown()
	})

	if s.configServer != nil {
		// Stop manager client
		g.Go(func() error {
			return s.configServer.Close()
		})
	}
	g.Go(func() error {
		// Stop metrics server
		return s.httpServer.Shutdown(ctx)
	})

	g.Go(func() error {
		// Stop grpc server
		return s.grpcServer.Shutdown()
	})
	return g.Wait()
}
