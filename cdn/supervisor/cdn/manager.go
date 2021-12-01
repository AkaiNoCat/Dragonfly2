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

//go:generate mockgen -destination ../mocks/cdn/mock_cdn_manager.go -package cdn d7y.io/dragonfly/v2/cdn/supervisor/cdn Manager

package cdn

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/disk"   // nolint
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/hybrid" // nolint
	"d7y.io/dragonfly/v2/cdn/supervisor/progress"
	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/ratelimiter/limitreader"
	"d7y.io/dragonfly/v2/pkg/ratelimiter/ratelimiter"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem/server"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"d7y.io/dragonfly/v2/pkg/util/timeutils"
)

// Manager as an interface defines all operations against CDN and
// operates on the underlying files stored on the local disk, etc.
type Manager interface {

	// TriggerCDN will trigger the download resource from sourceURL.
	TriggerCDN(context.Context, *task.SeedTask) (*task.SeedTask, error)

	// Delete the cdn meta with specified taskID.
	// The file on the disk will be deleted when the force is true.
	Delete(taskID string) error

	// TryFreeSpace checks if the free space of the storage is larger than the fileLength.
	TryFreeSpace(fileLength int64) (bool, error)
}

// Ensure that Manager implements the CDNManager interface
var _ Manager = (*manager)(nil)

var tracer = otel.Tracer("cdn-server")

// Manager is an implementation of the interface of Manager.
type manager struct {
	cfg             Config
	cacheStore      storage.Manager
	limiter         *ratelimiter.RateLimiter
	cdnLocker       *synclock.LockerPool
	metadataManager *metadataManager
	progressManager progress.Manager
	taskManager     task.Manager
	cdnReporter     *reporter
	detector        *cacheDetector
	writer          *cacheWriter
}

// NewManager returns a new Manager.
func NewManager(cfg Config, cacheStore storage.Manager, progressManager progress.Manager,
	taskManager task.Manager) (Manager, error) {
	rateLimiter := ratelimiter.NewRateLimiter(ratelimiter.TransRate(int64(cfg.MaxBandwidth-cfg.SystemReservedBandwidth)), 2)
	metadataManager := newMetadataManager(cacheStore)
	cdnReporter := newReporter(progressManager)
	return &manager{
		cfg:             cfg.applyDefaults(),
		cacheStore:      cacheStore,
		limiter:         rateLimiter,
		metadataManager: metadataManager,
		cdnReporter:     cdnReporter,
		progressManager: progressManager,
		taskManager:     taskManager,
		detector:        newCacheDetector(metadataManager, cacheStore),
		writer:          newCacheWriter(cdnReporter, metadataManager, cacheStore),
		cdnLocker:       synclock.NewLockerPool(),
	}, nil
}

func (cm *manager) TriggerCDN(ctx context.Context, seedTask *task.SeedTask) (*task.SeedTask, error) {
	updateTaskInfo, err := cm.doTrigger(ctx, seedTask)
	if err != nil {
		seedTask.Log().Errorf("failed to trigger cdn: %v", err)
		// todo source not reach error SOURCE_ERROR
		updateTaskInfo = getUpdateTaskInfoWithStatusOnly(seedTask, task.StatusFailed)
	}
	err = cm.progressManager.PublishTask(ctx, seedTask.ID, updateTaskInfo)
	return updateTaskInfo, err
}

func (cm *manager) doTrigger(ctx context.Context, seedTask *task.SeedTask) (*task.SeedTask, error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanTriggerCDN)
	defer span.End()
	cm.cdnLocker.Lock(seedTask.ID, false)
	defer cm.cdnLocker.UnLock(seedTask.ID, false)

	var fileDigest = md5.New()
	var digestType = digestutils.Md5Hash.String()
	if !stringutils.IsBlank(seedTask.Digest) {
		requestDigest := digestutils.Parse(seedTask.Digest)
		digestType = requestDigest[0]
		fileDigest = digestutils.CreateHash(digestType)
	}
	// first: detect Cache
	detectResult, err := cm.detector.detectCache(ctx, seedTask, fileDigest)
	if err != nil {
		return nil, errors.Wrap(err, "detect task cache")
	}
	span.SetAttributes(config.AttributeCacheResult.String(detectResult.String()))
	seedTask.Log().Infof("detects cache result: %+v", detectResult)
	// second: report detect result
	err = cm.cdnReporter.reportDetectResult(ctx, seedTask.ID, detectResult)
	if err != nil {
		seedTask.Log().Errorf("failed to report detect cache result: %v", err)
		return nil, errors.Wrapf(err, "report detect cache result")
	}
	// full cache
	if detectResult.breakPoint == -1 {
		seedTask.Log().Infof("cache full hit on local")
		return getUpdateTaskInfo(seedTask, task.StatusSuccess, detectResult.fileMetadata.SourceRealDigest, detectResult.fileMetadata.PieceMd5Sign,
			detectResult.fileMetadata.SourceFileLen, detectResult.fileMetadata.CdnFileLength, detectResult.fileMetadata.TotalPieceCount), nil
	}
	server.StatSeedStart(seedTask.ID, seedTask.RawURL)
	start := time.Now()
	// third: start to download the source file
	var downloadSpan trace.Span
	ctx, downloadSpan = tracer.Start(ctx, config.SpanDownloadSource)
	downloadSpan.End()
	respBody, err := cm.download(ctx, seedTask, detectResult.breakPoint)
	// download fail
	if err != nil {
		downloadSpan.RecordError(err)
		server.StatSeedFinish(seedTask.ID, seedTask.RawURL, false, err, start, time.Now(), 0, 0)
		return nil, errors.Wrap(err, "download task file data")
	}
	defer respBody.Close()
	reader := limitreader.NewLimitReaderWithLimiterAndDigest(respBody, cm.limiter, fileDigest, digestutils.Algorithms[digestType])

	// forth: write to storage
	downloadMetadata, err := cm.writer.startWriter(ctx, reader, seedTask, detectResult.breakPoint)
	if err != nil {
		server.StatSeedFinish(seedTask.ID, seedTask.RawURL, false, err, start, time.Now(), downloadMetadata.backSourceLength,
			downloadMetadata.realSourceFileLength)
		return nil, errors.Wrap(err, "write task file data")
	}
	server.StatSeedFinish(seedTask.ID, seedTask.RawURL, true, nil, start, time.Now(), downloadMetadata.backSourceLength,
		downloadMetadata.realSourceFileLength)
	// fifth: handle CDN result
	err = cm.handleCDNResult(seedTask, downloadMetadata)
	if err != nil {
		return nil, err
	}
	return getUpdateTaskInfo(seedTask, task.StatusSuccess, downloadMetadata.sourceRealDigest, downloadMetadata.pieceMd5Sign,
		downloadMetadata.realSourceFileLength, downloadMetadata.realCdnFileLength, downloadMetadata.totalPieceCount), nil
}

func (cm *manager) Delete(taskID string) error {
	cm.cdnLocker.Lock(taskID, false)
	defer cm.cdnLocker.UnLock(taskID, false)
	err := cm.cacheStore.DeleteTask(taskID)
	if err != nil {
		return errors.Wrap(err, "failed to delete task files")
	}
	return nil
}

func (cm *manager) TryFreeSpace(fileLength int64) (bool, error) {
	return cm.cacheStore.TryFreeSpace(fileLength)
}

func (cm *manager) handleCDNResult(seedTask *task.SeedTask, downloadMetadata *downloadMetadata) error {
	seedTask.Log().Debugf("start handle cdn result, downloadMetadata: %+v", downloadMetadata)
	var success = true
	var errMsg string
	// check md5
	if !stringutils.IsBlank(seedTask.Digest) && seedTask.Digest != downloadMetadata.sourceRealDigest {
		errMsg = fmt.Sprintf("file digest not match expected: %s real: %s", seedTask.Digest, downloadMetadata.sourceRealDigest)
		success = false
	}
	// check source length
	if success && seedTask.SourceFileLength >= 0 && seedTask.SourceFileLength != downloadMetadata.realSourceFileLength {
		errMsg = fmt.Sprintf("file length not match expected: %d real: %d", seedTask.SourceFileLength, downloadMetadata.realSourceFileLength)
		success = false
	}
	if success && seedTask.TotalPieceCount > 0 && downloadMetadata.totalPieceCount != seedTask.TotalPieceCount {
		errMsg = fmt.Sprintf("task total piece count not match expected: %d real: %d", seedTask.TotalPieceCount, downloadMetadata.totalPieceCount)
		success = false
	}
	sourceFileLen := seedTask.SourceFileLength
	if success && seedTask.SourceFileLength <= 0 {
		sourceFileLen = downloadMetadata.realSourceFileLength
	}
	if err := cm.metadataManager.updateStatusAndResult(seedTask.ID, &storage.FileMetadata{
		Finish:           true,
		Success:          success,
		SourceFileLen:    sourceFileLen,
		CdnFileLength:    downloadMetadata.realCdnFileLength,
		SourceRealDigest: downloadMetadata.sourceRealDigest,
		TotalPieceCount:  downloadMetadata.totalPieceCount,
		PieceMd5Sign:     downloadMetadata.pieceMd5Sign,
	}); err != nil {
		return errors.Wrapf(err, "update metadata")
	}
	if !success {
		return errors.New(errMsg)
	}
	return nil
}

func (cm *manager) updateExpireInfo(taskID string, expireInfo map[string]string) {
	if err := cm.metadataManager.updateExpireInfo(taskID, expireInfo); err != nil {
		logger.WithTaskID(taskID).Errorf("failed to update expireInfo(%s): %v", expireInfo, err)
	}
	logger.WithTaskID(taskID).Infof("success to update expireInfo(%s)", expireInfo)
}

/*
	helper functions
*/
var getCurrentTimeMillisFunc = timeutils.CurrentTimeMillis

func getUpdateTaskInfoWithStatusOnly(seedTask *task.SeedTask, cdnStatus string) *task.SeedTask {
	cloneTask := seedTask.Clone()
	cloneTask.CdnStatus = cdnStatus
	return cloneTask
}

func getUpdateTaskInfo(seedTask *task.SeedTask, cdnStatus, realMD5, pieceMd5Sign string, sourceFileLength, cdnFileLength int64,
	totalPieceCount int32) *task.SeedTask {
	cloneTask := seedTask.Clone()
	cloneTask.SourceFileLength = sourceFileLength
	cloneTask.CdnFileLength = cdnFileLength
	cloneTask.CdnStatus = cdnStatus
	cloneTask.TotalPieceCount = totalPieceCount
	cloneTask.SourceRealDigest = realMD5
	cloneTask.PieceMd5Sign = pieceMd5Sign
	return cloneTask
}
