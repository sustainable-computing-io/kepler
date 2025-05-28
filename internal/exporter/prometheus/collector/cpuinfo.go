// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
)

// procFS is an interface for CPUInfo.
type procFS interface {
	CPUInfo() ([]procfs.CPUInfo, error)
}

type realProcFS struct {
	fs procfs.FS
}

func (r *realProcFS) CPUInfo() ([]procfs.CPUInfo, error) {
	return r.fs.CPUInfo()
}

func newProcFS(mountPoint string) (procFS, error) {
	fs, err := procfs.NewFS(mountPoint)
	if err != nil {
		return nil, err
	}
	return &realProcFS{fs: fs}, nil
}

// cpuInfoCollector collects CPU info metrics from procfs.
type cpuInfoCollector struct {
	sync.Mutex

	fs   procFS
	desc *prom.Desc
}

// NewCPUInfoCollector creates a CPUInfoCollector using a procfs mount path.
func NewCPUInfoCollector(procPath string) (*cpuInfoCollector, error) {
	fs, err := newProcFS(procPath)
	if err != nil {
		return nil, fmt.Errorf("creating procfs failed: %w", err)
	}
	return newCPUInfoCollectorWithFS(fs), nil
}

// newCPUInfoCollectorWithFS injects a procFS interface
func newCPUInfoCollectorWithFS(fs procFS) *cpuInfoCollector {
	return &cpuInfoCollector{
		fs: fs,
		desc: prom.NewDesc(
			prom.BuildFQName(keplerNS, "node", "cpu_info"),
			"CPU information from procfs",
			[]string{"processor", "vendor_id", "model_name", "physical_id", "core_id"},
			nil,
		),
	}
}

func (c *cpuInfoCollector) Describe(ch chan<- *prom.Desc) {
	ch <- c.desc
}

func (c *cpuInfoCollector) Collect(ch chan<- prom.Metric) {
	c.Lock()
	defer c.Unlock()

	cpuInfos, err := c.fs.CPUInfo()
	if err != nil {
		return
	}
	for _, ci := range cpuInfos {
		ch <- prom.MustNewConstMetric(
			c.desc,
			prom.GaugeValue,
			1,
			fmt.Sprintf("%d", ci.Processor),
			ci.VendorID,
			ci.ModelName,
			ci.PhysicalID,
			ci.CoreID,
		)
	}
}
