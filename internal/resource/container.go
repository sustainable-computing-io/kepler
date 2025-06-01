// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
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

// matchResult stores information about a successful regex match.
type matchResult struct {
	Runtime  ContainerRuntime
	ID       string
	StartIdx int // The starting index of the match in the original string
	MatchLen int // The length of the overall matched string
}

// containerInfoFromCgroupPaths iterates through cgroup paths, finds all possible matches,
// and selects the "deepest" match (i.e., the one that starts latest in the string).
func containerInfoFromCgroupPaths(paths []string) (ContainerRuntime, string) {
	var bestMatch *matchResult

	for _, path := range paths {
		var currentPathMatches []matchResult

		// Find all matches for the current path
		for pattern, runtime := range containerPatterns {
			// FindAllStringSubmatchIndex returns all successive matches of the expression,
			// returning the start and end indices of the match and its subexpressions.
			matches := pattern.FindAllStringSubmatchIndex(path, -1)
			if len(matches) > 0 {
				for _, match := range matches {
					// match[0] is start index of overall match, match[1] is end index of overall match
					// match[2] is start index of first capturing group, match[3] is end index of first capturing group
					if len(match) >= 4 { // Ensure there's a capturing group
						id := path[match[2]:match[3]]
						currentPathMatches = append(currentPathMatches, matchResult{
							Runtime:  runtime,
							ID:       id,
							StartIdx: match[0],
							MatchLen: match[1] - match[0],
						})
					}
				}
			}
		}

		// If multiple matches are found for the current path, pick the "deepest" one.
		// "Deepest" is defined as the match with the highest starting index.
		if len(currentPathMatches) > 0 {
			sort.Slice(currentPathMatches, func(i, j int) bool {
				// Sort by StartIdx in descending order to get the deepest match first.
				return currentPathMatches[i].StartIdx > currentPathMatches[j].StartIdx
			})

			// The first element after sorting will be the deepest match for this path.
			// Compare it with the overall bestMatch found so far across all paths.
			if bestMatch == nil || currentPathMatches[0].StartIdx > bestMatch.StartIdx {
				bestMatch = &currentPathMatches[0]
			}
		}
	}

	if bestMatch != nil {
		return bestMatch.Runtime, bestMatch.ID
	}

	return UnknownRuntime, "" // No match found
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
