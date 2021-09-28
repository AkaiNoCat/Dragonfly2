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
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"d7y.io/dragonfly/v2/internal/dfpath"
	"d7y.io/dragonfly/v2/pkg/source"
)

func init() {
	flag.StringVar(&dfpath.PluginsDir, "plugin-dir", ".", "")
}

func main() {
	flag.Parse()

	client, err := source.LoadPlugin("dfs")
	if err != nil {
		fmt.Printf("load plugin error: %s\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	l, err := client.GetContentLength(ctx, "", nil, nil)
	if err != nil {
		fmt.Printf("get content length error: %s\n", err)
		os.Exit(1)
	}

	rc, err := client.Download(ctx, "", nil, nil)
	if err != nil {
		fmt.Printf("download error: %s\n", err)
		os.Exit(1)
	}

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		fmt.Printf("read error: %s\n", err)
		os.Exit(1)
	}

	if l != int64(len(data)) {
		fmt.Printf("content length mismatch\n")
		os.Exit(1)
	}

	err = rc.Close()
	if err != nil {
		fmt.Printf("close error: %s\n", err)
		os.Exit(1)
	}
}
