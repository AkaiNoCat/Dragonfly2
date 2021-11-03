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

package dfget

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"d7y.io/dragonfly/v2/client/config"
	"d7y.io/dragonfly/v2/internal/idgen"
	"d7y.io/dragonfly/v2/pkg/source"
	sourcemock "d7y.io/dragonfly/v2/pkg/source/mock"
	"d7y.io/dragonfly/v2/pkg/util/digestutils"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func Test_downloadFromSource(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	assert.Nil(t, err)
	output := filepath.Join(homeDir, idgen.UUIDString())
	defer os.Remove(output)

	content := idgen.UUIDString()

	sourceClient := sourcemock.NewMockResourceClient(gomock.NewController(t))
	source.Register("http", sourceClient, func(request *source.Request) *source.Request {
		return request
	})
	defer source.UnRegister("http")

	cfg := &config.DfgetConfig{
		URL:    "http://a.b.c/xx",
		Output: output,
		Digest: strings.Join([]string{digestutils.Sha256Hash.String(), digestutils.Sha256(content)}, ":"),
	}
	request, err := source.NewRequest(cfg.URL)
	assert.Nil(t, err)
	sourceClient.EXPECT().Download(request).Return(ioutil.NopCloser(strings.NewReader(content)), nil)

	err = downloadFromSource(context.Background(), cfg, nil)
	assert.Nil(t, err)
}
