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

	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"k8s.io/klog/v2"
)

type ContainerStats struct {
	Stats

	PIDS          map[uint64]bool
	ContainerID   string
	ContainerName string
	PodName       string
	Namespace     string

	CgroupStatHandler cgroup.CCgroupStatHandler
	CgroupStatMap     map[string]*types.UInt64StatCollection
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
		CgroupStatMap: make(map[string]*types.UInt64StatCollection),
	}

	if config.ExposeCgroupMetrics {
		for _, metricName := range AvailableCGroupMetrics {
			c.CgroupStatMap[metricName] = types.NewUInt64StatCollection()
		}
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
	return fmt.Sprintf("energy from pod/container: name: %s/%s namespace: %s containerid:%s\n cgroupMetrics: %v\n",
		c.PodName,
		c.ContainerName,
		c.Namespace,
		c.ContainerID,
		c.CgroupStatMap,
	) + c.Stats.String()
}

func (c *ContainerStats) UpdateCgroupMetrics() error {
	if c.CgroupStatHandler == nil {
		return nil
	}
	err := c.CgroupStatHandler.SetCGroupStat(c.ContainerID, c.CgroupStatMap)
	if err != nil {
		klog.V(3).Infof("Error reading cgroup stats for container %s (%s): %v", c.ContainerName, c.ContainerID, err)
	}
	return err
}
