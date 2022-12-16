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

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/acpi"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"

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
		NodeMetrics:            *collector_metric.NewNodeMetrics(),
		ContainersMetrics:      map[string]*collector_metric.ContainerMetrics{},
		systemProcessName:      utils.SystemProcessName,
		systemProcessNamespace: utils.SystemProcessNamespace,
	}
	return c
}

func (c *Collector) Initialize() error {
	m, err := attacher.AttachBPFAssets()
	if err != nil {
		return fmt.Errorf("failed to attach bpf assets: %v", err)
	}
	c.bpfHCMeter = m

	err = cgroup.Init(c.ContainersMetrics)
	if err != nil {
		klog.V(5).Infoln(err)
		return err
	}
	c.updateNodeEnergyMetrics()
	c.acpiPowerMeter.Run(attacher.HardwareCountersEnabled)
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
	// we first updates the bpf which is resposible to include new containers in the ContainersMetrics collection
	// the bpf collects metrics per processes and then map the process ids to container ids
	// TODO: when bpf is not running, the ContainersMetrics will not be updated with new containers.
	// The ContainersMetrics will only have the containers that were identified during the initialization (initContainersMetrics)
	c.updateBPFMetrics() // collect new hardware counter metrics if possible

	// TODO: collect cgroup metrics only from cgroup to avoid unnecessary overhead to kubelet
	c.updateCgroupMetrics()  // collect new cgroup metrics from cgroup
	c.updateKubeletMetrics() // collect new cgroup metrics from kubelet

	if config.EnabledGPU && accelerator.IsGPUCollectionSupported() {
		c.updateAcceleratorMetrics()
	}

	// use the container's resource usage metrics to update the node metrics
	c.updateNodeResourceUsage()
	c.updateNodeEnergyMetrics()

	// calculate the container energy consumption using its resource utilization and the node components energy consumption
	c.updateContainerEnergy()

	// check the log verbosity level before iterating in all container
	if klog.V(3).Enabled() {
		for _, v := range c.ContainersMetrics {
			klog.V(3).Infoln(v)
		}
		klog.V(3).Infoln(c.NodeMetrics.String())
	}
	klog.V(2).Infof("Collector Update elapsed time: %s", time.Since(start))
}

// resetCurrValue reset existing podEnergy previous curr value
func (c *Collector) resetCurrValue() {
	for _, v := range c.ContainersMetrics {
		v.ResetCurr()
	}
	c.NodeMetrics.ResetCurr()
}
