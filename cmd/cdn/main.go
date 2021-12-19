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

package main

import (
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/disk"   //nolint:gci    // Register disk storage manager
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/hybrid" // Register hybrid storage manager
	_ "d7y.io/dragonfly/v2/pkg/source/hdfsprotocol"           // Register hdfs client
	_ "d7y.io/dragonfly/v2/pkg/source/httpprotocol"           // Register http client
	_ "d7y.io/dragonfly/v2/pkg/source/ossprotocol"            // Register oss client
	_ "d7y.io/dragonfly/v2/pkg/source/proxyprotocol"          // Register proxy client

	"d7y.io/dragonfly/v2/cmd/cdn/cmd" //nolint:gci
)

func main() {
	cmd.Execute()
}
