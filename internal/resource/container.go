// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	dockerPattern     = regexp.MustCompile(`/docker[-/]([0-9a-f]{64})`)
	containerdPattern = regexp.MustCompile(`/containerd[-/]([0-9a-f]{64})`)

	criContainerdPattern = regexp.MustCompile(`[:/]cri-containerd[-:]([0-9a-f]{64})`)
	crioPattern          = regexp.MustCompile(`/crio-([0-9a-f]{64})`)

	libpodPattern        = regexp.MustCompile(`libpod-([0-9a-f]{64}).*`)
	libpodPayloadPattern = regexp.MustCompile(`/libpod-payload-([0-9a-f]+)`)

	kubepodsPattern = regexp.MustCompile(`/kubepods/[^/]+/pod[0-9a-f\-]+/([0-9a-f]{64})`)
)

// containerPatterns maps pre-compiled patterns to runtime types
var containerPatterns = map[*regexp.Regexp]ContainerRuntime{
	dockerPattern:     DockerRuntime,
	containerdPattern: ContainerDRuntime,

	criContainerdPattern: ContainerDRuntime,
	crioPattern:          CrioRuntime,

	libpodPattern:        PodmanRuntime,
	libpodPayloadPattern: PodmanRuntime,

	kubepodsPattern: KubePodsRuntime,
}

// containerInfoFromProc detects if a process is running in a container and extracts container info
func containerInfoFromProc(proc procInfo) (*Container, error) {
	cgroups, err := proc.Cgroups()
	if err != nil {
		return nil, fmt.Errorf("failed to get process cgroups: %w", err)
	}

	if len(cgroups) == 0 {
		return nil, nil
	}

	// Check cgroups for container ID and runtime
	paths := make([]string, len(cgroups))
	for i, cg := range cgroups {
		paths[i] = cg.Path
	}
	runtime, ctnrID := containerInfoFromCgroupPaths(paths)
	if ctnrID == "" {
		// Not in a container
		return nil, nil
	}

	c := &Container{
		ID:      ctnrID,
		Runtime: runtime,
	}

	if env, err := proc.Environ(); err == nil {
		c.Name = containerNameFromEnv(env)
	}

	if c.Name == "" {
		// only parse cmdline if name from env did not work
		if cmdline, err := proc.CmdLine(); err == nil {
			c.Name = containerNameFromCmdLine(cmdline)
		}
	}

	return c, nil
}

// containerInfoFromCgroupPaths looks for container IDs in cgroup paths
func containerInfoFromCgroupPaths(paths []string) (ContainerRuntime, string) {
	for _, path := range paths {
		for pattern, runtime := range containerPatterns {
			if matches := pattern.FindStringSubmatch(path); len(matches) > 1 {
				return runtime, matches[1]
			}
		}
	}

	return UnknownRuntime, ""
}

// containerNameFromEnv extracts container metadata from environment variables
func containerNameFromEnv(env []string) string {
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "HOSTNAME", "CONTAINER_NAME":
			return value
		}
	}

	return ""
}

// containerNameFromCmdLine returns container name from command line using --name flag
func containerNameFromCmdLine(cmdline []string) string {
	if len(cmdline) <= 1 {
		return ""
	}

	exe := filepath.Base(cmdline[0])

	// look for --name flag
	for i, arg := range cmdline {
		if i > 0 {
			// Handle --name=value format
			if strings.HasPrefix(arg, "--name=") {
				return strings.TrimPrefix(arg, "--name=")
			}

			// Handle --name value format (two separate arguments)
			if arg == "--name" && i+1 < len(cmdline) {
				return cmdline[i+1]
			}
		}

		// Container runtime might pass the container name as an arg
		if (exe == "docker-containerd-shim" || exe == "containerd-shim") && i == 3 {
			return arg
		}
	}

	return ""
}
