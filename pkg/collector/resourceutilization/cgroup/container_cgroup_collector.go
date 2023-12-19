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

package cgroup

import (
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	"k8s.io/klog/v2"
)

// UpdateContainerCgroupMetrics adds container-level cgroup data
func UpdateContainerCgroupMetrics(containerStats map[string]*stats.ContainerStats) {
	for key := range containerStats {
		if containerStats[key].CgroupStatHandler == nil {
			for pid := range containerStats[key].PIDS {
				if pid == 0 {
					klog.V(3).Infof("PID 0 does not have cgroup metrics since there is no /proc/0/cgroup")
					continue
				}
				handler, err := cgroup.NewCGroupStatManager(int(pid))
				if err != nil {
					klog.V(3).Infof("Error: could not start cgroup stat handler for PID %d: %v", pid, err)
					continue
				}
				containerStats[key].CgroupStatHandler = handler
				break
			}
		}
		// we need to test again in case the cgroup handler could not be created, e.g., for system processes
		if containerStats[key].CgroupStatHandler != nil {
			if err := containerStats[key].UpdateCgroupMetrics(); err != nil {
				// if the cgroup metrics of a container does not exist, it means that the container was deleted
				if key != utils.SystemProcessName && strings.Contains(err.Error(), "cgroup deleted") {
					delete(containerStats, key)
					klog.V(1).Infof("Container/Pod %s/%s was removed from the map because the cgroup was deleted",
						containerStats[key].ContainerName, containerStats[key].PodName)
				}
			}
		}
	}
}
