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

	"github.com/sustainable-computing-io/kepler/pkg/collector/metricdefine"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

type CCgroupV1StatHandler struct {
	statsHandler *statsv1.Metrics
}

type CCgroupV2StatHandler struct {
	statsHandler *statsv2.Metrics
}

// NewCGroupStatHandler creates a new cgroup stat object that can return the current metrics of the cgroup
// To avoid casting of interfaces, the CCgroupStatHandler has a handler for cgroup V1 or V2.
// See comments here: https://github.com/sustainable-computing-io/kepler/pull/609#discussion_r1155043868
func NewCGroupStatHandler(pid int) (CCgroupStatHandler, error) {
	p := fmt.Sprintf(procPath, pid)
	_, path, err := cgroups.ParseCgroupFileUnified(p)
	if err != nil {
		return nil, err
	}

	if config.GetCGroupVersion() == 1 {
		manager, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(path))
		if err != nil {
			return nil, err
		}
		v1StatHandler, err := manager.Stat(cgroups.IgnoreNotExist)
		if err != nil {
			return nil, err
		}
		return CCgroupV1StatHandler{
			statsHandler: v1StatHandler,
		}, nil
	} else {
		str := strings.Split(path, "/")
		size := len(str)
		slice := strings.Join(str[0:size-1], "/") + "/"
		group := str[size-1]
		manager, err := cgroup2.LoadSystemd(slice, group)
		if err != nil {
			return nil, err
		}
		v2StatHandler, err := manager.Stat()
		if err != nil {
			return nil, err
		}
		return CCgroupV2StatHandler{
			statsHandler: v2StatHandler,
		}, nil
	}
}

func (handler CCgroupV1StatHandler) GetCGroupStat(containerID string, cgroupStatMap map[string]*metricdefine.UInt64StatCollection) {
	// cgroup v1 memory
	cgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerID, handler.statsHandler.Memory.Usage.Usage)
	cgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerID, handler.statsHandler.Memory.Kernel.Usage)
	cgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerID, handler.statsHandler.Memory.KernelTCP.Usage)
	// cgroup v1 cpu
	cgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerID, handler.statsHandler.CPU.Usage.Total/1000)        // Usec
	cgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerID, handler.statsHandler.CPU.Usage.Kernel/1000) // Usec
	cgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerID, handler.statsHandler.CPU.Usage.User/1000)     // Usec
	// cgroup v1 IO
	for _, ioEntry := range handler.statsHandler.Blkio.IoServiceBytesRecursive {
		if ioEntry.Op == "Read" {
			cgroupStatMap[config.CgroupfsReadIO].AddDeltaStat(containerID, ioEntry.Value)
		}
		if ioEntry.Op == "Write" {
			cgroupStatMap[config.CgroupfsWriteIO].AddDeltaStat(containerID, ioEntry.Value)
		}
		cgroupStatMap[config.BlockDevicesIO].AddDeltaStat(containerID, 1)
	}
}

func (handler CCgroupV2StatHandler) GetCGroupStat(containerID string, cgroupStatMap map[string]*metricdefine.UInt64StatCollection) {
	// memory
	cgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerID, handler.statsHandler.Memory.Usage)
	cgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerID, handler.statsHandler.Memory.KernelStack)
	cgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerID, handler.statsHandler.Memory.Sock)
	// cpu
	cgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerID, handler.statsHandler.CPU.UsageUsec)
	cgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerID, handler.statsHandler.CPU.SystemUsec)
	cgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerID, handler.statsHandler.CPU.UserUsec)
	// IO
	for _, ioEntry := range handler.statsHandler.Io.GetUsage() {
		cgroupStatMap[config.CgroupfsReadIO].AddDeltaStat(containerID, ioEntry.Rbytes)
		cgroupStatMap[config.CgroupfsWriteIO].AddDeltaStat(containerID, ioEntry.Wbytes)
		cgroupStatMap[config.BlockDevicesIO].AddDeltaStat(containerID, 1)
	}
}
