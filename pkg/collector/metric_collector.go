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
	"fmt"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/power/acpi"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	maxInactiveContainers = 10
)

type Collector struct {
	// instance that collects the bpf metrics
	bpfHCMeter *attacher.BpfModuleTables
	// instance that collects the node energy consumption
	acpiPowerMeter *acpi.ACPI

	// TODO: fix me: these metrics should be in NodeMetrics structure
	NodeSensorEnergy map[string]float64
	NodePkgEnergy    map[int]source.RAPLEnergy
	NodeGPUEnergy    []uint32
	NodeCPUFrequency map[int32]uint64

	// NodeMetrics holds all node energy and resource usage metrics
	NodeMetrics collector_metric.NodeMetrics

	// ContainersMetrics holds all container energy and resource usage metrics
	ContainersMetrics map[string]*collector_metric.ContainerMetrics

	// generic names to be used for process that are not within a pod
	systemProcessName      string
	systemProcessNamespace string
}

func NewCollector() *Collector {
	c := &Collector{
		acpiPowerMeter:         acpi.NewACPIPowerMeter(),
		NodeSensorEnergy:       map[string]float64{},
		NodePkgEnergy:          map[int]source.RAPLEnergy{},
		NodeGPUEnergy:          []uint32{},
		NodeCPUFrequency:       map[int32]uint64{},
		NodeMetrics:            *collector_metric.NewNodeMetrics(),
		ContainersMetrics:      map[string]*collector_metric.ContainerMetrics{},
		systemProcessName:      utils.GetSystemProcessName(),
		systemProcessNamespace: utils.GetSystemProcessNamespace(),
	}
	return c
}

func (c *Collector) Initialize() error {
	m, err := attacher.AttachBPFAssets()
	if err != nil {
		return fmt.Errorf("failed to attach bpf assets: %v", err)
	}
	c.bpfHCMeter = m

	pods, err := cgroup.Init()
	if err != nil {
		klog.V(5).Infoln(err)
		return err
	}

	c.prePopulateContainerMetrics(pods)
	c.updateMeasuredNodeEnergy()
	c.NodeMetrics.SetValues(c.NodeSensorEnergy, c.NodePkgEnergy, c.NodeGPUEnergy, [][]float64{}) // set initial energy
	c.acpiPowerMeter.Run()
	c.resetBPFTables()

	return nil
}

func (c *Collector) Destroy() {
	if c.bpfHCMeter != nil {
		attacher.DetachBPFModules(c.bpfHCMeter)
	}
}

// Update updates the node and container energy and resource usage metrics
func (c *Collector) Update() {
	start := time.Now()

	// reset the previous collected value because not all containers will have new data
	// that is, a container that was inactive will not have any update but we need to set its metrics to 0
	c.resetCurrValue()

	// update container metrics regarding the resource utilization to be used to calculate the energy consumption
	c.updateContainerResourceUsageMetrics()
	containerMetricValuesOnly, containerIDList, containerGPUDelta := c.getContainerMetricsList()

	// use the container's resource usage metrics to update the node metrics
	nodeTotalPower, nodeTotalGPUPower, nodeTotalPowerPerComponents := c.updateNoderMetrics(containerMetricValuesOnly)

	// calculate the container energy consumption using its resource utilization and the node components energy consumption
	// TODO: minimize the number of collection variables, we already have the containerMetrics and NodeMetrics structures
	c.updateContainerEnergy(containerMetricValuesOnly, containerIDList, containerGPUDelta, nodeTotalPower, nodeTotalGPUPower, nodeTotalPowerPerComponents)

	// check the log verbosity level before iterating in all container
	if klog.V(3).Enabled() {
		for _, v := range c.ContainersMetrics {
			klog.V(3).Infoln(v)
		}
		klog.V(3).Infoln(c.NodeMetrics)
	}
	klog.V(2).Infoln("Collector Update elapsed time: %s", time.Since(start))
}

// resetCurrValue reset existing podEnergy previous curr value
func (c *Collector) resetCurrValue() {
	for _, v := range c.ContainersMetrics {
		v.ResetCurr()
	}
	c.NodeMetrics.ResetCurr()
}

// init adds the information of containers that were already running before kepler has been created
// This is a necessary hacking to export metrics of idle containers, since we can only include in
// the containers list the containers that present any updates from the bpf metrics
func (c *Collector) prePopulateContainerMetrics(pods *[]corev1.Pod) {
	for i := 0; i < len(*pods); i++ {
		pod := (*pods)[i]
		for j := 0; j < len(pod.Status.InitContainerStatuses); j++ {
			container := pod.Status.InitContainerStatuses[j]
			containerID := cgroup.ParseContainerIDFromPodStatus(container.ContainerID)
			c.ContainersMetrics[containerID] = collector_metric.NewContainerMetrics(container.Name, pod.Name, pod.Namespace)
		}
		for j := 0; j < len(pod.Status.ContainerStatuses); j++ {
			container := pod.Status.ContainerStatuses[j]
			containerID := cgroup.ParseContainerIDFromPodStatus(container.ContainerID)
			c.ContainersMetrics[containerID] = collector_metric.NewContainerMetrics(container.Name, pod.Name, pod.Namespace)
		}
		for j := 0; j < len(pod.Status.EphemeralContainerStatuses); j++ {
			container := pod.Status.EphemeralContainerStatuses[j]
			containerID := cgroup.ParseContainerIDFromPodStatus(container.ContainerID)
			c.ContainersMetrics[containerID] = collector_metric.NewContainerMetrics(container.Name, pod.Name, pod.Namespace)
		}
	}
}
