// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collectors

import (
	"bufio"
	"os/exec"
	"strings"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"
)

const (
	architecture     = "architecture"
	model_name       = "model_name"
	cpus             = "cpus"
	cores_per_socket = "cores_per_socket"
	sockets          = "sockets"
	vendor_id        = "vendor_id"
)

// cmdRunner defines an interface for running commands.
type cmdRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// execRunner implements CommandRunner using exec.Command.
type execRunner struct{}

func (e *execRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

type cpuInfo struct {
	sync.Mutex
	sync.Once
	sysroot  string
	desc     *prom.Desc
	cache    map[string]string
	cacheErr error

	commandRunner cmdRunner
}

var _ prom.Collector = &cpuInfo{}

// NewCpuInfoCollector creates a new CPU info collector.
// The sysroot parameter specifies the path to the root filesystem (e.g., /host in a container),
// allowing lscpu to read CPU information from the host's /proc and /sys when mounted.
func NewCpuInfoCollector(sysroot string) *cpuInfo {
	return &cpuInfo{
		sysroot: sysroot,
		desc: prom.NewDesc(
			prom.BuildFQName(namespace, "", "cpu_info"),
			"CPU information from lscpu command",
			[]string{
				architecture,
				model_name,
				cpus,
				cores_per_socket,
				sockets,
				vendor_id,
			},
			nil,
		),
		cache:         make(map[string]string),
		commandRunner: &execRunner{},
	}
}

func (ci *cpuInfo) Describe(ch chan<- *prom.Desc) {
	ch <- ci.desc
}

func (ci *cpuInfo) Collect(ch chan<- prom.Metric) {
	ci.Lock()
	defer ci.Unlock()

	// Populate cpuinfo cache once
	ci.Do(func() {
		ci.cache, ci.cacheErr = ci.getCPUInfo()
	})

	// Skip metric emission if initialization failed
	if ci.cacheErr != nil {
		return
	}

	// Use cached values
	ch <- prom.MustNewConstMetric(
		ci.desc,
		prom.GaugeValue,
		1.0,
		ci.cache[architecture],
		ci.cache[model_name],
		ci.cache[cpus],
		ci.cache[cores_per_socket],
		ci.cache[sockets],
		ci.cache[vendor_id],
	)
}

func (ci *cpuInfo) getCPUInfo() (map[string]string, error) {
	kv, err := ci.runLscpu()
	if err != nil {
		return nil, err
	}
	return ci.processCPUInfo(kv), nil
}

func (ci *cpuInfo) runLscpu() (map[string]string, error) {
	var output []byte
	var err error
	if ci.sysroot != "" {
		output, err = ci.commandRunner.Run("lscpu", "--sysroot", ci.sysroot)
	} else {
		output, err = ci.commandRunner.Run("lscpu")
	}
	if err != nil {
		return nil, err
	}

	kv := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		kv[key] = value
	}

	return kv, nil
}

func (ci *cpuInfo) processCPUInfo(kv map[string]string) map[string]string {
	info := make(map[string]string)
	info[architecture] = ci.getCPUArch(kv)
	info[model_name] = ci.getCPUModelName(kv)
	info[cpus] = ci.getCPUs(kv)
	info[cores_per_socket] = ci.getCoresPerSocket(kv)
	info[sockets] = ci.getSockets(kv)
	info[vendor_id] = ci.getVendorID(kv)
	return info
}

func (ci *cpuInfo) getCPUArch(kv map[string]string) string {
	if val, ok := kv["Architecture"]; ok {
		return val
	}
	return "unknown"
}

func (ci *cpuInfo) getCPUModelName(kv map[string]string) string {
	if val, ok := kv["Model name"]; ok {
		return val
	}
	return "unknown"
}

func (ci *cpuInfo) getCPUs(kv map[string]string) string {
	if val, ok := kv["CPU(s)"]; ok {
		return val
	}
	return "unknown"
}

func (ci *cpuInfo) getCoresPerSocket(kv map[string]string) string {
	if val, ok := kv["Core(s) per socket"]; ok {
		return val
	}
	return "unknown"
}

func (ci *cpuInfo) getSockets(kv map[string]string) string {
	if val, ok := kv["Socket(s)"]; ok {
		return val
	}
	return "unknown"
}

func (ci *cpuInfo) getVendorID(kv map[string]string) string {
	if val, ok := kv["Vendor ID"]; ok {
		return val
	}
	return "unknown"
}
