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

package e2eutil

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

const (
	kindDockerContainer = "kind-control-plane"
)

func DockerCommand(arg ...string) *exec.Cmd {
	extArgs := []string{"exec", "-i", kindDockerContainer}
	extArgs = append(extArgs, arg...)
	return exec.Command("docker", extArgs...)
}

func CriCtlCommand(arg ...string) *exec.Cmd {
	extArgs := []string{"/usr/local/bin/crictl"}
	extArgs = append(extArgs, arg...)
	return DockerCommand(extArgs...)
}

func KubeCtlCommand(arg ...string) *exec.Cmd {
	return exec.Command("kubectl", arg...)
}

func ABCommand(arg ...string) *exec.Cmd {
	return exec.Command("ab", arg...)
}

func GitCommand(arg ...string) *exec.Cmd {
	return exec.Command("git", arg...)
}

type PodExec struct {
	namespace string
	name      string
	container string
}

func NewPodExec(namespace string, name string, container string) *PodExec {
	return &PodExec{
		namespace: namespace,
		name:      name,
		container: container,
	}
}

func (p *PodExec) Command(arg ...string) *exec.Cmd {
	extArgs := []string{"-n", p.namespace, "exec", p.name, "--"}
	if p.container != "" {
		extArgs = []string{"-n", p.namespace, "exec", "-c", p.container, p.name, "--"}
	}
	extArgs = append(extArgs, arg...)
	return KubeCtlCommand(extArgs...)
}

func (p *PodExec) CurlCommand(method string, header map[string]string, data map[string]interface{}, target string) *exec.Cmd {
	extArgs := []string{"/usr/bin/curl", target, "-s"}
	if method != "" {
		extArgs = append(extArgs, "-X", method)
	}
	if header != nil {
		for k, v := range header {
			extArgs = append(extArgs, "-H", fmt.Sprintf("%s:%s", k, v))
		}
	}
	if data != nil {
		jsonData, _ := json.Marshal(data)
		extArgs = append(extArgs, "-d", string(jsonData))
	}
	return p.Command(extArgs...)
}

func KubeCtlCopyCommand(ns, pod, source, target string) *exec.Cmd {
	return KubeCtlCommand("-n", ns, "cp", pod+":"+source, target)
}
