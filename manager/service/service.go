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

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"

	"d7y.io/dragonfly/v2/manager/cache"
	"d7y.io/dragonfly/v2/manager/database"
	"d7y.io/dragonfly/v2/manager/job"
	"d7y.io/dragonfly/v2/manager/model"
	"d7y.io/dragonfly/v2/manager/permission/rbac"
	"d7y.io/dragonfly/v2/manager/types"
)

type REST interface {
	GetUser(context.Context, uint) (*model.User, error)
	GetUsers(context.Context, types.GetUsersQuery) (*[]model.User, int64, error)
	SignIn(context.Context, types.SignInRequest) (*model.User, error)
	SignUp(context.Context, types.SignUpRequest) (*model.User, error)
	OauthSignin(context.Context, string) (string, error)
	OauthSigninCallback(context.Context, string, string) (*model.User, error)
	ResetPassword(context.Context, uint, types.ResetPasswordRequest) error
	GetRolesForUser(context.Context, uint) ([]string, error)
	AddRoleForUser(context.Context, types.AddRoleForUserParams) (bool, error)
	DeleteRoleForUser(context.Context, types.DeleteRoleForUserParams) (bool, error)

	CreateRole(context.Context, types.CreateRoleRequest) error
	DestroyRole(context.Context, string) (bool, error)
	GetRole(context.Context, string) [][]string
	GetRoles(context.Context) []string
	AddPermissionForRole(context.Context, string, types.AddPermissionForRoleRequest) (bool, error)
	DeletePermissionForRole(context.Context, string, types.DeletePermissionForRoleRequest) (bool, error)

	GetPermissions(context.Context, *gin.Engine) []rbac.Permission

	CreateOauth(context.Context, types.CreateOauthRequest) (*model.Oauth, error)
	DestroyOauth(context.Context, uint) error
	UpdateOauth(context.Context, uint, types.UpdateOauthRequest) (*model.Oauth, error)
	GetOauth(context.Context, uint) (*model.Oauth, error)
	GetOauths(context.Context, types.GetOauthsQuery) (*[]model.Oauth, int64, error)

	CreateCDNCluster(context.Context, types.CreateCDNClusterRequest) (*model.CDNCluster, error)
	DestroyCDNCluster(context.Context, uint) error
	UpdateCDNCluster(context.Context, uint, types.UpdateCDNClusterRequest) (*model.CDNCluster, error)
	GetCDNCluster(context.Context, uint) (*model.CDNCluster, error)
	GetCDNClusters(context.Context, types.GetCDNClustersQuery) (*[]model.CDNCluster, int64, error)
	AddCDNToCDNCluster(context.Context, uint, uint) error
	AddSchedulerClusterToCDNCluster(context.Context, uint, uint) error

	CreateCDN(context.Context, types.CreateCDNRequest) (*model.CDN, error)
	DestroyCDN(context.Context, uint) error
	UpdateCDN(context.Context, uint, types.UpdateCDNRequest) (*model.CDN, error)
	GetCDN(context.Context, uint) (*model.CDN, error)
	GetCDNs(context.Context, types.GetCDNsQuery) (*[]model.CDN, int64, error)

	CreateSchedulerCluster(context.Context, types.CreateSchedulerClusterRequest) (*model.SchedulerCluster, error)
	DestroySchedulerCluster(context.Context, uint) error
	UpdateSchedulerCluster(context.Context, uint, types.UpdateSchedulerClusterRequest) (*model.SchedulerCluster, error)
	GetSchedulerCluster(context.Context, uint) (*model.SchedulerCluster, error)
	GetSchedulerClusters(context.Context, types.GetSchedulerClustersQuery) (*[]model.SchedulerCluster, int64, error)
	AddSchedulerToSchedulerCluster(context.Context, uint, uint) error

	CreateScheduler(context.Context, types.CreateSchedulerRequest) (*model.Scheduler, error)
	DestroyScheduler(context.Context, uint) error
	UpdateScheduler(context.Context, uint, types.UpdateSchedulerRequest) (*model.Scheduler, error)
	GetScheduler(context.Context, uint) (*model.Scheduler, error)
	GetSchedulers(context.Context, types.GetSchedulersQuery) (*[]model.Scheduler, int64, error)

	CreateSecurityRule(context.Context, types.CreateSecurityRuleRequest) (*model.SecurityRule, error)
	DestroySecurityRule(context.Context, uint) error
	UpdateSecurityRule(context.Context, uint, types.UpdateSecurityRuleRequest) (*model.SecurityRule, error)
	GetSecurityRule(context.Context, uint) (*model.SecurityRule, error)
	GetSecurityRules(context.Context, types.GetSecurityRulesQuery) (*[]model.SecurityRule, int64, error)

	CreateSecurityGroup(context.Context, types.CreateSecurityGroupRequest) (*model.SecurityGroup, error)
	DestroySecurityGroup(context.Context, uint) error
	UpdateSecurityGroup(context.Context, uint, types.UpdateSecurityGroupRequest) (*model.SecurityGroup, error)
	GetSecurityGroup(context.Context, uint) (*model.SecurityGroup, error)
	GetSecurityGroups(context.Context, types.GetSecurityGroupsQuery) (*[]model.SecurityGroup, int64, error)
	AddSchedulerClusterToSecurityGroup(context.Context, uint, uint) error
	AddCDNClusterToSecurityGroup(context.Context, uint, uint) error
	AddSecurityRuleToSecurityGroup(context.Context, uint, uint) error
	DestroySecurityRuleToSecurityGroup(context.Context, uint, uint) error

	CreateConfig(context.Context, types.CreateConfigRequest) (*model.Config, error)
	DestroyConfig(context.Context, uint) error
	UpdateConfig(context.Context, uint, types.UpdateConfigRequest) (*model.Config, error)
	GetConfig(context.Context, uint) (*model.Config, error)
	GetConfigs(context.Context, types.GetConfigsQuery) (*[]model.Config, int64, error)

	CreatePreheatJob(context.Context, types.CreatePreheatJobRequest) (*model.Job, error)
	DestroyJob(context.Context, uint) error
	UpdateJob(context.Context, uint, types.UpdateJobRequest) (*model.Job, error)
	GetJob(context.Context, uint) (*model.Job, error)
	GetJobs(context.Context, types.GetJobsQuery) (*[]model.Job, int64, error)

	CreateV1Preheat(context.Context, types.CreateV1PreheatRequest) (*types.CreateV1PreheatResponse, error)
	GetV1Preheat(context.Context, string) (*types.GetV1PreheatResponse, error)

	CreateApplication(context.Context, types.CreateApplicationRequest) (*model.Application, error)
	DestroyApplication(context.Context, uint) error
	UpdateApplication(context.Context, uint, types.UpdateApplicationRequest) (*model.Application, error)
	GetApplication(context.Context, uint) (*model.Application, error)
	GetApplications(context.Context, types.GetApplicationsQuery) (*[]model.Application, int64, error)
	AddSchedulerClusterToApplication(context.Context, uint, uint) error
	DeleteSchedulerClusterToApplication(context.Context, uint, uint) error
	AddCDNClusterToApplication(context.Context, uint, uint) error
	DeleteCDNClusterToApplication(context.Context, uint, uint) error
}

type rest struct {
	db       *gorm.DB
	rdb      *redis.Client
	cache    *cache.Cache
	job      *job.Job
	enforcer *casbin.Enforcer
}

// NewREST returns a new REST instence
func NewREST(database *database.Database, cache *cache.Cache, job *job.Job, enforcer *casbin.Enforcer) REST {
	return &rest{
		db:       database.DB,
		rdb:      database.RDB,
		cache:    cache,
		job:      job,
		enforcer: enforcer,
	}
}
