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
	"d7y.io/dragonfly/v2/cmd/cdn/cmd"

	_ "d7y.io/dragonfly/v2/cdn/storedriver/local"             // register disk driver
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/disk"   // register disk storage manager
	_ "d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage/hybrid" // register hybrid storage manager
	_ "d7y.io/dragonfly/v2/pkg/source/httpprotocol"           // register HTTP client
	_ "d7y.io/dragonfly/v2/pkg/source/ossprotocol"            // register OSS client
)

func main() {
	cmd.Execute()
}
