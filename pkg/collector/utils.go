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

package collector

import (
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
)

func (c *Collector) createContainersMetricsIfNotExist(containerID string, cGroupID, pid uint64, withCGroupID bool) {
	if _, ok := c.ContainersMetrics[containerID]; !ok {
		info, _ := cgroup.GetContainerInfo(cGroupID, pid, withCGroupID)
		c.ContainersMetrics[containerID] = collector_metric.NewContainerMetrics(info.ContainerName, info.PodName, info.Namespace, containerID)
	}
}

func (c *Collector) createProcessMetricsIfNotExist(pid uint64, command string) {
	if p, ok := c.ProcessMetrics[pid]; !ok {
		c.ProcessMetrics[pid] = collector_metric.NewProcessMetrics(pid, command)
	} else if p.Command == "" {
		p.Command = command
	}
}
