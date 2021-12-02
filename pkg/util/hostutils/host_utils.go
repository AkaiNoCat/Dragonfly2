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

package hostutils

import (
	"os"

	"github.com/Showmax/go-fqdn"
)

var Hostname string
var FQDNHostname string

func init() {
	Hostname = hostname()
	FQDNHostname = fqdnHostname()
}

// Get kernel hostname
func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	return name
}

// Get FQDN hostname
func fqdnHostname() string {
	fqdn, _ := fqdn.FqdnHostname()
	//if err != nil {
	//	panic(err)
	//}

	return fqdn
}
