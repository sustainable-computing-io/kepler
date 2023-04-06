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
	"github.com/containerd/cgroups/v3/cgroup2"

	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"k8s.io/klog/v2"
)

type CCgroupV1StatManager struct {
	manager cgroups.Cgroup
}

type CCgroupV12StatManager struct {
	manager *cgroup2.Manager
}

// NewCGroupStatManager creates a new cgroup stat object that can return the current metrics of the cgroup
// To avoid casting of interfaces, the CCgroupStatHandler has a handler for cgroup V1 or V2.
// See comments here: https://github.com/sustainable-computing-io/kepler/pull/609#discussion_r1155043868
func NewCGroupStatManager(pid int) (CCgroupStatHandler, error) {
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
		return CCgroupV1StatManager{
			manager: manager,
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
		return CCgroupV12StatManager{
			manager: manager,
		}, nil
	}
}

func (c CCgroupV1StatManager) SetCGroupStat(containerID string, cgroupStatMap map[string]*types.UInt64StatCollection) error {
	stat, err := c.manager.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return err
	}
	// cgroup v1 memory
	cgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerID, stat.Memory.Usage.Usage)
	cgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerID, stat.Memory.Kernel.Usage)
	cgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerID, stat.Memory.KernelTCP.Usage)
	// cgroup v1 cpu
	cgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerID, stat.CPU.Usage.Total/1000) // Usec

	klog.Infoln(containerID, stat.CPU.Usage.Total/1000)
	cgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerID, stat.CPU.Usage.Kernel/1000) // Usec
	cgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerID, stat.CPU.Usage.User/1000)     // Usec
	// cgroup v1 IO
	for _, ioEntry := range stat.Blkio.IoServiceBytesRecursive {
		if ioEntry.Op == "Read" {
			cgroupStatMap[config.CgroupfsReadIO].AddDeltaStat(containerID, ioEntry.Value)
		}
		if ioEntry.Op == "Write" {
			cgroupStatMap[config.CgroupfsWriteIO].AddDeltaStat(containerID, ioEntry.Value)
		}
		cgroupStatMap[config.BlockDevicesIO].AddDeltaStat(containerID, 1)
	}
	return nil
}

func (c CCgroupV12StatManager) SetCGroupStat(containerID string, cgroupStatMap map[string]*types.UInt64StatCollection) error {
	stat, err := c.manager.Stat()
	if err != nil {
		return err
	}
	// memory
	cgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerID, stat.Memory.Usage)
	cgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerID, stat.Memory.KernelStack)
	cgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerID, stat.Memory.Sock)
	// cpu
	cgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerID, stat.CPU.UsageUsec)
	cgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerID, stat.CPU.SystemUsec)
	cgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerID, stat.CPU.UserUsec)
	// IO
	for _, ioEntry := range stat.Io.GetUsage() {
		cgroupStatMap[config.CgroupfsReadIO].AddDeltaStat(containerID, ioEntry.Rbytes)
		cgroupStatMap[config.CgroupfsWriteIO].AddDeltaStat(containerID, ioEntry.Wbytes)
		cgroupStatMap[config.BlockDevicesIO].AddDeltaStat(containerID, 1)
	}
	return nil
}
