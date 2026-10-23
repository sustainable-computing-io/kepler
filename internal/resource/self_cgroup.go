// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"os"
	"strings"
)

// SelfCgroupFile is where a process reads its own cgroup membership.
const SelfCgroupFile = "/proc/self/cgroup"

// InPrivateCgroupNamespace reports whether the process that reads the cgroup file at path
// runs in a private cgroup namespace, which is when every cgroup path it reads is the
// namespace root. In the host cgroup namespace (a privileged container, or any container
// on a cgroup v1 node) its own cgroup is a full path, and "/" is the host root cgroup.
func InPrivateCgroupNamespace(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", path, err)
	}

	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		// hierarchy-ID:controller-list:cgroup-path
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}
		found = true
		if parts[2] != cgroupNamespaceRoot {
			return false, nil
		}
	}
	return found, nil
}
