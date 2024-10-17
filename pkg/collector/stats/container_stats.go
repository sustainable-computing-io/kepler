/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package stats

import (
	"fmt"
)

type ContainerStats struct {
	Stats

	PIDS          map[uint64]bool
	ContainerID   string
	ContainerName string
	PodName       string
	Namespace     string
}

// NewContainerStats creates a new ContainerStats instance
func NewContainerStats(containerName, podName, podNamespace, containerID string) *ContainerStats {
	c := &ContainerStats{
		Stats:         *NewStats(),
		PIDS:          make(map[uint64]bool),
		ContainerID:   containerID,
		PodName:       podName,
		ContainerName: containerName,
		Namespace:     podNamespace,
	}

	return c
}

// ResetCurr reset all current value to 0
func (c *ContainerStats) ResetDeltaValues() {
	c.Stats.ResetDeltaValues()
	c.PIDS = map[uint64]bool{}
}

// SetLatestProcess set PID to the latest captured process
// NOTICE: can lose main container info for multi-container pod
func (c *ContainerStats) SetLatestProcess(pid uint64) {
	c.PIDS[pid] = true
}

func (c *ContainerStats) String() string {
	return fmt.Sprintf("energy from pod/container: name: %s/%s namespace: %s containerid:%s\n",
		c.PodName,
		c.ContainerName,
		c.Namespace,
		c.ContainerID,
	) + c.Stats.String()
}
