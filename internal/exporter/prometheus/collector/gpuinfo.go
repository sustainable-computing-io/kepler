// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// gpuInfoCollector collects GPU device info metrics.
type gpuInfoCollector struct {
	sync.Mutex

	pm       PowerDataProvider
	desc     *prom.Desc
	nodeName string
}

// NewGPUInfoCollector creates a GPUInfoCollector that exports GPU device information.
func NewGPUInfoCollector(pm PowerDataProvider, nodeName string) *gpuInfoCollector {
	return &gpuInfoCollector{
		pm:       pm,
		nodeName: nodeName,
		desc: prom.NewDesc(
			prom.BuildFQName(keplerNS, "node", "gpu_info"),
			"GPU device information for mapping index to UUID/name",
			[]string{"gpu", "gpu_uuid", "gpu_name", "vendor"},
			prom.Labels{nodeNameLabel: nodeName},
		),
	}
}

func (c *gpuInfoCollector) Describe(ch chan<- *prom.Desc) {
	ch <- c.desc
}

func (c *gpuInfoCollector) Collect(ch chan<- prom.Metric) {
	c.Lock()
	defer c.Unlock()

	snapshot, err := c.pm.Snapshot()
	if err != nil {
		return
	}

	c.collectGPUInfo(ch, snapshot.GPUStats)
}

func (c *gpuInfoCollector) collectGPUInfo(ch chan<- prom.Metric, gpuStats []monitor.GPUDeviceStats) {
	for _, stats := range gpuStats {
		ch <- prom.MustNewConstMetric(
			c.desc,
			prom.GaugeValue,
			1,
			fmt.Sprintf("%d", stats.DeviceIndex),
			stats.UUID,
			stats.Name,
			stats.Vendor,
		)
	}
}
