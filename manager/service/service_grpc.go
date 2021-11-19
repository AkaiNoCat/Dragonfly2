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

package service

import (
	"context"
	"errors"
	"io"

	cachev8 "github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/manager/cache"
	"d7y.io/dragonfly/v2/manager/database"
	"d7y.io/dragonfly/v2/manager/model"
	"d7y.io/dragonfly/v2/manager/searcher"
	"d7y.io/dragonfly/v2/pkg/rpc/manager"
)

type GRPC struct {
	db    *gorm.DB
	rdb   *redis.Client
	cache *cache.Cache
	manager.UnimplementedManagerServer
	searcher searcher.Searcher
}

// NewREST returns a new REST instence
func NewGRPC(database *database.Database, cache *cache.Cache, searcher searcher.Searcher) *GRPC {
	return &GRPC{
		db:       database.DB,
		rdb:      database.RDB,
		cache:    cache,
		searcher: searcher,
	}
}

func (s *GRPC) GetCDN(ctx context.Context, req *manager.GetCDNRequest) (*manager.CDN, error) {
	var pbCDN manager.CDN
	cacheKey := cache.MakeCDNCacheKey(req.HostName, uint(req.CdnClusterId))

	// Cache Hit
	if err := s.cache.Get(ctx, cacheKey, &pbCDN); err == nil {
		logger.Infof("%s cache hit", cacheKey)
		return &pbCDN, nil
	}

	// Cache Miss
	logger.Infof("%s cache miss", cacheKey)
	cdn := model.CDN{}
	if err := s.db.WithContext(ctx).Preload("CDNCluster").First(&cdn, &model.CDN{
		HostName:     req.HostName,
		CDNClusterID: uint(req.CdnClusterId),
	}).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	config, err := cdn.CDNCluster.Config.MarshalJSON()
	if err != nil {
		return nil, status.Error(codes.DataLoss, err.Error())
	}

	pbCDN = manager.CDN{
		Id:           uint64(cdn.ID),
		HostName:     cdn.HostName,
		Idc:          cdn.IDC,
		Location:     cdn.Location,
		Ip:           cdn.IP,
		Port:         cdn.Port,
		DownloadPort: cdn.DownloadPort,
		State:        cdn.State,
		CdnClusterId: uint64(cdn.CDNClusterID),
		CdnCluster: &manager.CDNCluster{
			Id:     uint64(cdn.CDNCluster.ID),
			Name:   cdn.CDNCluster.Name,
			Bio:    cdn.CDNCluster.BIO,
			Config: config,
		},
	}

	if err := s.cache.Once(&cachev8.Item{
		Ctx:   ctx,
		Key:   cacheKey,
		Value: &pbCDN,
		TTL:   s.cache.TTL,
	}); err != nil {
		logger.Warnf("storage cache failed: %v", err)
	}

	return &pbCDN, nil
}

func (s *GRPC) createCDN(ctx context.Context, req *manager.UpdateCDNRequest) (*manager.CDN, error) {
	cdn := model.CDN{
		HostName:     req.HostName,
		IDC:          req.Idc,
		Location:     req.Location,
		IP:           req.Ip,
		Port:         req.Port,
		DownloadPort: req.DownloadPort,
		CDNClusterID: uint(req.CdnClusterId),
	}

	if err := s.db.WithContext(ctx).Create(&cdn).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &manager.CDN{
		Id:           uint64(cdn.ID),
		HostName:     cdn.HostName,
		Location:     cdn.Location,
		Ip:           cdn.IP,
		Port:         cdn.Port,
		DownloadPort: cdn.DownloadPort,
		CdnClusterId: uint64(cdn.CDNClusterID),
		State:        cdn.State,
	}, nil
}

func (s *GRPC) UpdateCDN(ctx context.Context, req *manager.UpdateCDNRequest) (*manager.CDN, error) {
	cdn := model.CDN{}
	if err := s.db.WithContext(ctx).First(&cdn, model.CDN{
		HostName:     req.HostName,
		CDNClusterID: uint(req.CdnClusterId),
	}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.createCDN(ctx, req)
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}

	if err := s.db.WithContext(ctx).Model(&cdn).Updates(model.CDN{
		IDC:          req.Idc,
		Location:     req.Location,
		IP:           req.Ip,
		Port:         req.Port,
		DownloadPort: req.DownloadPort,
		CDNClusterID: uint(req.CdnClusterId),
	}).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	if err := s.cache.Delete(
		ctx,
		cache.MakeCDNCacheKey(cdn.HostName, cdn.CDNClusterID),
	); err != nil {
		logger.Warnf("%s refresh keepalive status failed in cdn cluster %d", cdn.HostName, cdn.CDNClusterID)
	}

	return &manager.CDN{
		Id:           uint64(cdn.ID),
		HostName:     cdn.HostName,
		Location:     cdn.Location,
		Ip:           cdn.IP,
		Port:         cdn.Port,
		DownloadPort: cdn.DownloadPort,
		CdnClusterId: uint64(cdn.CDNClusterID),
		State:        cdn.State,
	}, nil
}

func (s *GRPC) GetScheduler(ctx context.Context, req *manager.GetSchedulerRequest) (*manager.Scheduler, error) {
	var pbScheduler manager.Scheduler
	cacheKey := cache.MakeSchedulerCacheKey(req.HostName, uint(req.SchedulerClusterId))

	// Cache Hit
	if err := s.cache.Get(ctx, cacheKey, &pbScheduler); err == nil {
		logger.Infof("%s cache hit", cacheKey)
		return &pbScheduler, nil
	}

	// Cache Miss
	logger.Infof("%s cache miss", cacheKey)
	scheduler := model.Scheduler{}
	if err := s.db.WithContext(ctx).Preload("SchedulerCluster").Preload("SchedulerCluster.CDNClusters.CDNs", &model.CDN{
		State: model.CDNStateActive,
	}).First(&scheduler, &model.Scheduler{
		HostName:           req.HostName,
		SchedulerClusterID: uint(req.SchedulerClusterId),
	}).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	schedulerNetConfig, err := scheduler.NetConfig.MarshalJSON()
	if err != nil {
		return nil, status.Error(codes.DataLoss, err.Error())
	}

	schedulerClusterConfig, err := scheduler.SchedulerCluster.Config.MarshalJSON()
	if err != nil {
		return nil, status.Error(codes.DataLoss, err.Error())
	}

	schedulerClusterClientConfig, err := scheduler.SchedulerCluster.ClientConfig.MarshalJSON()
	if err != nil {
		return nil, status.Error(codes.DataLoss, err.Error())
	}

	var pbCDNs []*manager.CDN
	for _, cdnCluster := range scheduler.SchedulerCluster.CDNClusters {
		cdnClusterConfig, err := cdnCluster.Config.MarshalJSON()
		if err != nil {
			return nil, status.Error(codes.DataLoss, err.Error())
		}

		for _, cdn := range cdnCluster.CDNs {
			pbCDNs = append(pbCDNs, &manager.CDN{
				Id:           uint64(cdn.ID),
				HostName:     cdn.HostName,
				Idc:          cdn.IDC,
				Location:     cdn.Location,
				Ip:           cdn.IP,
				Port:         cdn.Port,
				DownloadPort: cdn.DownloadPort,
				CdnClusterId: uint64(cdn.CDNClusterID),
				CdnCluster: &manager.CDNCluster{
					Id:     uint64(cdnCluster.ID),
					Name:   cdnCluster.Name,
					Bio:    cdnCluster.BIO,
					Config: cdnClusterConfig,
				},
				State: cdn.State,
			})
		}
	}

	pbScheduler = manager.Scheduler{
		Id:                 uint64(scheduler.ID),
		HostName:           scheduler.HostName,
		Vips:               scheduler.VIPs,
		Idc:                scheduler.IDC,
		Location:           scheduler.Location,
		NetConfig:          schedulerNetConfig,
		Ip:                 scheduler.IP,
		Port:               scheduler.Port,
		State:              scheduler.State,
		SchedulerClusterId: uint64(scheduler.SchedulerClusterID),
		SchedulerCluster: &manager.SchedulerCluster{
			Id:           uint64(scheduler.SchedulerCluster.ID),
			Name:         scheduler.SchedulerCluster.Name,
			Bio:          scheduler.SchedulerCluster.BIO,
			Config:       schedulerClusterConfig,
			ClientConfig: schedulerClusterClientConfig,
		},
		Cdns: pbCDNs,
	}

	if err := s.cache.Once(&cachev8.Item{
		Ctx:   ctx,
		Key:   cacheKey,
		Value: &pbScheduler,
		TTL:   s.cache.TTL,
	}); err != nil {
		logger.Warnf("cache storage failed: %v", err)
	}

	return &pbScheduler, nil
}

func (s *GRPC) createScheduler(ctx context.Context, req *manager.UpdateSchedulerRequest) (*manager.Scheduler, error) {
	var netConfig model.JSONMap
	if len(req.NetConfig) > 0 {
		if err := netConfig.UnmarshalJSON(req.NetConfig); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	scheduler := model.Scheduler{
		HostName:           req.HostName,
		VIPs:               req.Vips,
		IDC:                req.Idc,
		Location:           req.Location,
		NetConfig:          netConfig,
		IP:                 req.Ip,
		Port:               req.Port,
		SchedulerClusterID: uint(req.SchedulerClusterId),
	}

	if err := s.db.WithContext(ctx).Create(&scheduler).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	return &manager.Scheduler{
		Id:                 uint64(scheduler.ID),
		HostName:           scheduler.HostName,
		Vips:               scheduler.VIPs,
		Idc:                scheduler.IDC,
		Location:           scheduler.Location,
		NetConfig:          req.NetConfig,
		Ip:                 scheduler.IP,
		Port:               scheduler.Port,
		State:              scheduler.State,
		SchedulerClusterId: uint64(scheduler.SchedulerClusterID),
	}, nil
}

func (s *GRPC) UpdateScheduler(ctx context.Context, req *manager.UpdateSchedulerRequest) (*manager.Scheduler, error) {
	scheduler := model.Scheduler{}
	if err := s.db.WithContext(ctx).First(&scheduler, model.Scheduler{
		HostName:           req.HostName,
		SchedulerClusterID: uint(req.SchedulerClusterId),
	}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.createScheduler(ctx, req)
		}
		return nil, status.Error(codes.Unknown, err.Error())
	}

	var netConfig model.JSONMap
	if len(req.NetConfig) > 0 {
		if err := netConfig.UnmarshalJSON(req.NetConfig); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	if err := s.db.WithContext(ctx).Model(&scheduler).Updates(model.Scheduler{
		VIPs:               req.Vips,
		IDC:                req.Idc,
		Location:           req.Location,
		NetConfig:          netConfig,
		IP:                 req.Ip,
		Port:               req.Port,
		SchedulerClusterID: uint(req.SchedulerClusterId),
	}).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	if err := s.cache.Delete(
		ctx,
		cache.MakeSchedulerCacheKey(scheduler.HostName, scheduler.SchedulerClusterID),
	); err != nil {
		logger.Warnf("%s refresh keepalive status failed in scheduler cluster %d", scheduler.HostName, scheduler.SchedulerClusterID)
	}

	return &manager.Scheduler{
		Id:                 uint64(scheduler.ID),
		HostName:           scheduler.HostName,
		Vips:               scheduler.VIPs,
		Idc:                scheduler.IDC,
		Location:           scheduler.Location,
		NetConfig:          req.NetConfig,
		Ip:                 scheduler.IP,
		Port:               scheduler.Port,
		SchedulerClusterId: uint64(scheduler.SchedulerClusterID),
		State:              scheduler.State,
	}, nil
}

func (s *GRPC) ListSchedulers(ctx context.Context, req *manager.ListSchedulersRequest) (*manager.ListSchedulersResponse, error) {
	var pbListSchedulersResponse manager.ListSchedulersResponse
	cacheKey := cache.MakeSchedulersCacheKey(req.HostName)

	// Cache Hit
	if err := s.cache.Get(ctx, cacheKey, &pbListSchedulersResponse); err == nil {
		logger.Infof("%s cache hit", cacheKey)
		return &pbListSchedulersResponse, nil
	}

	// Cache Miss
	logger.Infof("%s cache miss", cacheKey)
	var schedulerClusters []model.SchedulerCluster
	if err := s.db.WithContext(ctx).Preload("SecurityGroup.SecurityRules").Preload("Schedulers", "status = ?", "active").Find(&schedulerClusters).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	// Search optimal scheduler cluster
	schedulerCluster, ok := s.searcher.FindSchedulerCluster(schedulerClusters, req)
	if !ok {
		if err := s.db.WithContext(ctx).Find(&schedulerCluster, &model.SchedulerCluster{
			IsDefault: true,
		}).Error; err != nil {
			return nil, status.Error(codes.Unknown, err.Error())
		}
	}

	schedulers := []model.Scheduler{}
	if err := s.db.WithContext(ctx).Find(&schedulers, &model.Scheduler{
		State:              model.SchedulerStateActive,
		SchedulerClusterID: schedulerCluster.ID,
	}).Error; err != nil {
		return nil, status.Error(codes.Unknown, err.Error())
	}

	for _, scheduler := range schedulers {
		schedulerNetConfig, err := scheduler.NetConfig.MarshalJSON()
		if err != nil {
			return nil, status.Error(codes.DataLoss, err.Error())
		}

		pbListSchedulersResponse.Schedulers = append(pbListSchedulersResponse.Schedulers, &manager.Scheduler{
			Id:                 uint64(scheduler.ID),
			HostName:           scheduler.HostName,
			Vips:               scheduler.VIPs,
			Idc:                scheduler.IDC,
			Location:           scheduler.Location,
			NetConfig:          schedulerNetConfig,
			Ip:                 scheduler.IP,
			Port:               scheduler.Port,
			SchedulerClusterId: uint64(scheduler.SchedulerClusterID),
			State:              scheduler.State,
		})
	}

	if err := s.cache.Once(&cachev8.Item{
		Ctx:   ctx,
		Key:   cacheKey,
		Value: &pbListSchedulersResponse,
		TTL:   s.cache.TTL,
	}); err != nil {
		logger.Warnf("storage cache failed: %v", err)
	}

	return &pbListSchedulersResponse, nil
}

func (s *GRPC) KeepAlive(m manager.Manager_KeepAliveServer) error {
	req, err := m.Recv()
	if err != nil {
		logger.Errorf("keepalive failed for the first time: %v", err)
		return status.Error(codes.Unknown, err.Error())
	}
	hostName := req.HostName
	sourceType := req.SourceType
	clusterID := uint(req.ClusterId)
	logger.Infof("%s keepalive successfully for the first time in cluster %d", hostName, clusterID)

	// Active scheduler
	if sourceType == manager.SourceType_SCHEDULER_SOURCE {
		scheduler := model.Scheduler{}
		if err := s.db.First(&scheduler, model.Scheduler{
			HostName:           hostName,
			SchedulerClusterID: clusterID,
		}).Updates(model.Scheduler{
			State: model.SchedulerStateActive,
		}).Error; err != nil {
			return status.Error(codes.Unknown, err.Error())
		}

		if err := s.cache.Delete(
			context.TODO(),
			cache.MakeSchedulerCacheKey(hostName, clusterID),
		); err != nil {
			logger.Warnf("%s refresh keepalive status failed in scheduler cluster %d", hostName, clusterID)
		}
	}

	// Active CDN
	if sourceType == manager.SourceType_CDN_SOURCE {
		cdn := model.CDN{}
		if err := s.db.First(&cdn, model.CDN{
			HostName:     hostName,
			CDNClusterID: clusterID,
		}).Updates(model.CDN{
			State: model.CDNStateActive,
		}).Error; err != nil {
			return status.Error(codes.Unknown, err.Error())
		}

		if err := s.cache.Delete(
			context.TODO(),
			cache.MakeCDNCacheKey(hostName, clusterID),
		); err != nil {
			logger.Warnf("%s refresh keepalive status failed in cdn cluster %d", hostName, clusterID)
		}
	}

	for {
		_, err := m.Recv()
		if err != nil {
			// Inactive scheduler
			if sourceType == manager.SourceType_SCHEDULER_SOURCE {
				scheduler := model.Scheduler{}
				if err := s.db.First(&scheduler, model.Scheduler{
					HostName:           hostName,
					SchedulerClusterID: clusterID,
				}).Updates(model.Scheduler{
					State: model.SchedulerStateInactive,
				}).Error; err != nil {
					return status.Error(codes.Unknown, err.Error())
				}

				if err := s.cache.Delete(
					context.TODO(),
					cache.MakeSchedulerCacheKey(hostName, clusterID),
				); err != nil {
					logger.Warnf("%s refresh keepalive status failed in scheduler cluster %d", hostName, clusterID)
				}
			}

			// Inactive CDN
			if sourceType == manager.SourceType_CDN_SOURCE {
				cdn := model.CDN{}
				if err := s.db.First(&cdn, model.CDN{
					HostName:     hostName,
					CDNClusterID: clusterID,
				}).Updates(model.CDN{
					State: model.CDNStateInactive,
				}).Error; err != nil {
					return status.Error(codes.Unknown, err.Error())
				}

				if err := s.cache.Delete(
					context.TODO(),
					cache.MakeCDNCacheKey(hostName, clusterID),
				); err != nil {
					logger.Warnf("%s refresh keepalive status failed in cdn cluster %d", hostName, clusterID)
				}
			}

			if err == io.EOF {
				logger.Infof("%s close keepalive in cluster %d", hostName, clusterID)
				return nil
			}
			logger.Errorf("%s keepalive failed in cluster %d: %v", hostName, clusterID, err)
			return status.Error(codes.Unknown, err.Error())
		}

		logger.Debugf("%s type of %s send keepalive request in cluster %d", sourceType, hostName, clusterID)
	}
}
