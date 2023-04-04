//go:build linux
// +build linux

/*
Copyright 2023.

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
	"fmt"
	"strings"

	"github.com/containerd/cgroups"
	statsv1 "github.com/containerd/cgroups/stats/v1"
	"github.com/containerd/cgroups/v3/cgroup2"
	statsv2 "github.com/containerd/cgroups/v3/cgroup2/stats"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

type CCgroupStatHandler struct {
	cgroupV1StatHandler *statsv1.Metrics
	cgroupV2StatHandler *statsv2.Metrics
}

var (
	AvailableCGroupMetrics = []string{
		config.CgroupfsMemory, config.CgroupfsKernelMemory, config.CgroupfsTCPMemory,
		config.CgroupfsCPU, config.CgroupfsSystemCPU, config.CgroupfsUserCPU,
		config.CgroupfsReadIO, config.CgroupfsWriteIO, config.BlockDevicesIO,
	}
)

// NewCGroupStatHandler creates a new cgroup stat object that can return the current metrics of the cgroup
// To avoid casting of interfaces, the CCgroupStatHandler has a handler for cgroup V1 or V2.
// See comments here: https://github.com/sustainable-computing-io/kepler/pull/609#discussion_r1155043868
func NewCGroupStatHandler(pid int) (*CCgroupStatHandler, error) {
	hander := CCgroupStatHandler{}
	p := fmt.Sprintf(procPath, pid)
	_, path, err := cgroups.ParseCgroupFileUnified(p)
	if err != nil {
		return nil, err
	}

	if config.GetCGroupVersion() == 1 {
		manager, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(path))
		if err != nil {
			return &hander, err
		}
		v1StatHandler, err := manager.Stat(cgroups.IgnoreNotExist)
		if err != nil {
			return &hander, err
		}
		hander.cgroupV1StatHandler = v1StatHandler
	} else {
		str := strings.Split(path, "/")
		size := len(str)
		slice := strings.Join(str[0:size-1], "/") + "/"
		group := str[size-1]
		manager, err := cgroup2.LoadSystemd(slice, group)
		if err != nil {
			return &hander, err
		}
		v2StatHandler, err := manager.Stat()
		if err != nil {
			return &hander, err
		}
		hander.cgroupV2StatHandler = v2StatHandler
	}
	return &hander, nil
}

func GetAvailableCGroupMetrics() []string {
	return AvailableCGroupMetrics
}

func (hander *CCgroupStatHandler) GetCGroupStat() (stats map[string]uint64, err error) {
	statsMap := make(map[string]uint64)
	if config.GetCGroupVersion() == 1 {
		readCgroupV1MemoryStat(hander.cgroupV1StatHandler, statsMap)
		readCgroupV1CPUStat(hander.cgroupV1StatHandler, statsMap)
		readCgroupV1IOStat(hander.cgroupV1StatHandler, statsMap)
	} else {
		readCgroupV2MemoryStat(hander.cgroupV2StatHandler, statsMap)
		readCgroupV2CPUStat(hander.cgroupV2StatHandler, statsMap)
		readCgroupV2IOStat(hander.cgroupV2StatHandler, statsMap)
	}
	return statsMap, nil
}

func readCgroupV1MemoryStat(handler *statsv1.Metrics, statsMap map[string]uint64) {
	statsMap[config.CgroupfsMemory] = handler.Memory.Usage.Usage
	statsMap[config.CgroupfsKernelMemory] = handler.Memory.Kernel.Usage
	statsMap[config.CgroupfsTCPMemory] = handler.Memory.KernelTCP.Usage
}

func readCgroupV2MemoryStat(handler *statsv2.Metrics, statsMap map[string]uint64) {
	statsMap[config.CgroupfsMemory] = handler.Memory.Usage
	statsMap[config.CgroupfsKernelMemory] = handler.Memory.KernelStack
	statsMap[config.CgroupfsTCPMemory] = handler.Memory.Sock
}

func readCgroupV1CPUStat(handler *statsv1.Metrics, statsMap map[string]uint64) {
	statsMap[config.CgroupfsCPU] = handler.CPU.Usage.Total / 1000        // Usec
	statsMap[config.CgroupfsSystemCPU] = handler.CPU.Usage.Kernel / 1000 // Usec
	statsMap[config.CgroupfsUserCPU] = handler.CPU.Usage.User / 1000     // Usec
}

func readCgroupV2CPUStat(handler *statsv2.Metrics, statsMap map[string]uint64) {
	statsMap[config.CgroupfsCPU] = handler.CPU.UsageUsec
	statsMap[config.CgroupfsSystemCPU] = handler.CPU.SystemUsec
	statsMap[config.CgroupfsUserCPU] = handler.CPU.UserUsec
}

func readCgroupV1IOStat(handler *statsv1.Metrics, statsMap map[string]uint64) {
	// Each ioEntry is an io device.
	for _, ioEntry := range handler.Blkio.IoServiceBytesRecursive {
		if ioEntry.Op == "Read" {
			statsMap[config.CgroupfsReadIO] += ioEntry.Value
		}
		if ioEntry.Op == "Write" {
			statsMap[config.CgroupfsWriteIO] += ioEntry.Value
		}
		statsMap[config.BlockDevicesIO] += 1
	}
}

func readCgroupV2IOStat(handler *statsv2.Metrics, statsMap map[string]uint64) {
	// Each ioEntry is an io device.
	for _, ioEntry := range handler.Io.GetUsage() {
		statsMap[config.CgroupfsReadIO] += ioEntry.Rbytes
		statsMap[config.CgroupfsWriteIO] += ioEntry.Wbytes
		statsMap[config.BlockDevicesIO] += 1
	}
}
