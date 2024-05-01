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

package accelerator

import (
	"fmt"
	"os"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/libvirt"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	dev "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

const (
	procPath string = "/proc/%d/cgroup"
)

var (
	// lastUtilizationTimestamp represents the CPU timestamp in microseconds at which utilization samples were last read
	lastUtilizationTimestamp time.Time = time.Now()
)

// UpdateProcessGPUUtilizationMetrics reads the GPU metrics of each process using the GPU
func UpdateProcessGPUUtilizationMetrics(processStats map[uint64]*stats.ProcessStats, bpfSupportedMetrics bpf.SupportedMetrics) {
	if gpus, err := acc.GetActiveAcceleratorsByType("gpu"); err == nil {
		for _, a := range gpus {
			d := a.GetAccelerator()
			migDevices := d.GetDeviceInstances()
			for _, _device := range d.GetDevicesByID() {
				// we need to use MIG device handler if the GPU has MIG slices, otherwise, we use the GPU device handler
				if _, hasMIG := migDevices[_device.(dev.GPUDevice).ID]; hasMIG {
					// if the device has MIG slices, we should collect the process information directly from the MIG device handler
					for _, migDevice := range migDevices[_device.(dev.GPUDevice).ID] {
						// device.ID is equal to migDevice.ParentID
						// we add the process metrics with the parent GPU ID, so that the Ratio power model will use this data to split the GPU power among the process
						addGPUUtilizationToProcessStats(d, processStats, migDevice.(dev.GPUDevice), migDevice.(dev.GPUDevice).ParentID, bpfSupportedMetrics)
					}
				} else {
					addGPUUtilizationToProcessStats(d, processStats, _device.(dev.GPUDevice), _device.(dev.GPUDevice).ID, bpfSupportedMetrics)
				}
			}
		}
	}
	lastUtilizationTimestamp = time.Now()
}

func addGPUUtilizationToProcessStats(ai dev.AcceleratorInterface, processStats map[uint64]*stats.ProcessStats, d dev.GPUDevice, gpuID int, bpfSupportedMetrics bpf.SupportedMetrics) {
	var err error
	var processesUtilization map[uint32]any

	if processesUtilization, err = ai.GetProcessResourceUtilizationPerDevice(d, time.Since(lastUtilizationTimestamp)); err != nil {
		klog.Infoln(err)
		return
	}

	for pid, processUtilization := range processesUtilization {
		uintPid := uint64(pid)
		// if the process was not indentified by the bpf metrics, create a new metric object
		if _, exist := processStats[uintPid]; !exist {
			command := getProcessCommand(uintPid)
			containerID := utils.SystemProcessName

			// if the pid is within a container, it will have an container ID
			if config.IsExposeContainerStatsEnabled() {
				if containerID, err = cgroup.GetContainerIDFromPID(uintPid); err != nil {
					klog.V(6).Infof("failed to resolve container for Pid %v (command=%s): %v, set containerID=%s", pid, command, err, containerID)
				}
			}

			// if the pid is within a VM, it will have an VM ID
			vmID := utils.EmptyString
			if config.IsExposeVMStatsEnabled() {
				if config.IsExposeVMStatsEnabled() {
					vmID, err = libvirt.GetVMID(uintPid)
					if err != nil {
						klog.V(6).Infof("failed to resolve VM ID for PID %v (command=%s): %v", pid, command, err)
					}
				}
			}
			processStats[uintPid] = stats.NewProcessStats(uintPid, uint64(0), containerID, vmID, command, bpfSupportedMetrics)
		}
		gpuName := fmt.Sprintf("%d", gpuID) // GPU ID or Parent GPU ID for MIG slices
		processStats[uintPid].ResourceUsage[config.GPUComputeUtilization].AddDeltaStat(gpuName, uint64(processUtilization.(dev.GPUProcessUtilizationSample).ComputeUtil))
		processStats[uintPid].ResourceUsage[config.GPUMemUtilization].AddDeltaStat(gpuName, uint64(processUtilization.(dev.GPUProcessUtilizationSample).MemUtil))
	}
}

func getProcessCommand(pid uint64) string {
	fileName := fmt.Sprintf(procPath, pid)
	// Read the file
	comm, err := os.ReadFile(fileName)
	if err != nil {
		return ""
	}
	return string(comm)
}
