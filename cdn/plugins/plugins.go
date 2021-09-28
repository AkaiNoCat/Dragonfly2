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

package plugins

import (
	"fmt"

	logger "d7y.io/dragonfly/v2/internal/dflog"
)

var mgr = NewManager()

// Initialize builds all plugins defined in config file.
func Initialize(plugins map[PluginType][]*PluginProperties) error {
	for _, pt := range PluginTypes {
		for _, v := range plugins[pt] {
			if !v.Enable {
				logger.Infof("plugin[%s][%s] is disabled", pt, v.Name)
				continue
			}
			builder, ok := mgr.GetBuilder(pt, v.Name)
			if !ok {
				return fmt.Errorf("can not find builder to create plugin[%s][%s]", pt, v.Name)
			}
			p, err := builder(v.Config)
			if err != nil {
				return fmt.Errorf("failed to build plugin[%s][%s]: %v", pt, v.Name, err)
			}
			if err := mgr.AddPlugin(p); err != nil {
				return fmt.Errorf("failed to add plugin[%s][%s]: %v", pt, v.Name, err)
			}
			logger.Infof("add plugin[%s][%s] success", pt, v.Name)
		}
	}
	return nil
}

// RegisterPluginBuilder register a plugin builder that will be called to create a new
// plugin instant when cdn starts.
func RegisterPluginBuilder(pt PluginType, name string, builder Builder) error {
	return mgr.AddBuilder(pt, name, builder)
}

// GetPlugin returns a plugin instant with the giving plugin type and name.
func GetPlugin(pt PluginType, name string) (Plugin, bool) {
	return mgr.GetPlugin(pt, name)
}
