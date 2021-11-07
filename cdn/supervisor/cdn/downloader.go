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
	"fmt"
	"io"

	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/rangeutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
)

func (cm *Manager) download(ctx context.Context, task *types.SeedTask, breakPoint int64) (io.ReadCloser, error) {
	var err error
	breakRange := task.Range
	if breakPoint > 0 {
		// todo replace task.SourceFileLength with totalSourceFileLength to get BreakRange
		breakRange, err = getBreakRange(breakPoint, task.Range, task.SourceFileLength)
		if err != nil {
			return nil, errors.Wrapf(err, "calculate the breakRange")
		}
	}
	task.Log().Infof("start download url %s at range: %d-%d: with header: %+v", task.RawURL, breakPoint,
		task.SourceFileLength, task.Range)
	downloadRequest, err := source.NewRequestWithHeader(task.RawURL, task.Header)
	if err != nil {
		return nil, errors.Wrap(err, "create download request")
	}
	if stringutils.IsBlank(breakRange) {
		downloadRequest.Header.Add(source.Range, breakRange)
	}
	body, expireInfo, err := source.DownloadWithExpireInfo(downloadRequest)
	// update Expire info
	if err == nil {
		cm.updateExpireInfo(task.ID, map[string]string{
			source.LastModified: expireInfo.LastModified,
			source.ETag:         expireInfo.ETag,
		})
	}
	return body, err
}

func getBreakRange(breakPoint int64, taskRange string, length int64) (string, error) {
	if breakPoint < 0 {
		return "", errors.Errorf("breakPoint is illegal, breakPoint: %d", breakPoint)
	}
	if stringutils.IsBlank(taskRange) {
		return fmt.Sprintf("%d-", breakPoint), nil
	}
	requestRange, err := rangeutils.ParseRange(taskRange, length)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%d", requestRange.StartIndex+breakPoint, requestRange.EndIndex), nil
}
