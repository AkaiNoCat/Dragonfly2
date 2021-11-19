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

package idgen

import (
	"strings"

	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"d7y.io/dragonfly/v2/pkg/util/net/urlutils"
)

// TaskID generates a taskId.
// filter is separated by & character.
func TaskID(url string, meta *base.UrlMeta) string {
	var data []string

	filter := ""
	if meta != nil {
		filter = meta.Filter
	}

	data = append(data, urlutils.FilterURLParam(url, strings.Split(filter, "&")))

	if meta != nil {
		if meta.Digest != "" {
			data = append(data, meta.Digest)
		}

		if meta.Range != "" {
			data = append(data, meta.Range)
		}

		if meta.Tag != "" {
			data = append(data, meta.Tag)
		}
	}

	return digestutils.Sha256(data...)
}
