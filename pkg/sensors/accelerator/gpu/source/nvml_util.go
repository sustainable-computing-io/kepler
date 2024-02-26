//go:build gpu
// +build gpu

/*
Copyright 2024.

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
package source

import (
	"encoding/xml"
	"fmt"
	"os/exec"

	"k8s.io/klog/v2"
)

type NvidiaSmiLog struct {
	XMLName       xml.Name `xml:"nvidia_smi_log"`
	Timestamp     string   `xml:"timestamp,omitempty"`
	DriverVersion string   `xml:"driver_version,omitempty"`
	CudaVersion   string   `xml:"cuda_version,omitempty"`
	AttachedGPUs  int      `xml:"attached_gpus,omitempty"`
	GPU           []GPU    `xml:"gpu"`
}

type GPU struct {
	ID         string      `xml:"id,attr"`
	MigMode    MigMode     `xml:"mig_mode,omitempty"`
	MigDevices []MigDevice `xml:"mig_devices>mig_device,omitempty"`
	UUID       string      `xml:"uuid,omitempty"`
}

type MigMode struct {
	CurrentMig string `xml:"current_mig,omitempty"`
	PendingMig string `xml:"pending_mig,omitempty"`
}

type MigDevice struct {
	GPUInstanceID            int              `xml:"gpu_instance_id,omitempty"`
	ComputeInstanceID        int              `xml:"compute_instance_id,omitempty"`
	DeviceAttributes         DeviceAttributes `xml:"device_attributes,omitempty"`
	EntityName               string           // this is set later
	MultiprocessorCountRatio float64          // this is set later
}

type DeviceAttributes struct {
	Shared SharedAttributes `xml:"shared,omitempty"`
}

type SharedAttributes struct {
	MultiprocessorCount int `xml:"multiprocessor_count,omitempty"`
}

// RetriveFromNvidiaSMI retrives the MIG information from nvidia-smi
func RetriveFromNvidiaSMI(debug bool) (gpuMigArray [][]MigDevice, totalMultiProcessorCount map[string]int, err error) {
	cmd := exec.Command("nvidia-smi", "-q", "-x")
	output, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("Error running nvidia-smi command:", err)
		return
	}

	var nvidiaSmiLog NvidiaSmiLog
	err = xml.Unmarshal(output, &nvidiaSmiLog)
	if err != nil {
		err = fmt.Errorf("Error unmarshaling XML:", err)
		return
	}

	gpuMigArray = make([][]MigDevice, len(nvidiaSmiLog.GPU))
	totalMultiProcessorCount = make(map[string]int, len(nvidiaSmiLog.GPU))
	for i, gpu := range nvidiaSmiLog.GPU {
		// find the largest GPUInstanceID among the MIGDevices, to make sure we have enough space in the array
		maxGPUInstanceID := 0
		for _, migDevice := range gpu.MigDevices {
			if migDevice.GPUInstanceID > maxGPUInstanceID {
				maxGPUInstanceID = migDevice.GPUInstanceID
			}
		}
		gpuMigArray[i] = make([]MigDevice, maxGPUInstanceID+1)
		totalMultiProcessorCount[gpu.UUID] = 0
		for _, migDevice := range gpu.MigDevices {
			gpuMigArray[i][migDevice.GPUInstanceID] = migDevice
			totalMultiProcessorCount[gpu.UUID] += migDevice.DeviceAttributes.Shared.MultiprocessorCount
		}
		// count MultiprocessorCountRatio for each device
		for j, migDevice := range gpuMigArray[i] {
			gpuMigArray[i][j].MultiprocessorCountRatio = float64(migDevice.DeviceAttributes.Shared.MultiprocessorCount) / float64(totalMultiProcessorCount[gpu.UUID])
		}
	}

	if debug {
		for i, gpu := range nvidiaSmiLog.GPU {
			for _, device := range gpuMigArray[i] {
				klog.Infof("GPU %d %q", i, gpu.UUID)
				klog.Infof("\tGPUInstanceID: %d\n", device.GPUInstanceID)
				klog.Infof("\tComputeInstanceID: %d\n", device.ComputeInstanceID)
				klog.Infof("\tShared MultiprocessorCount: %d\n", device.DeviceAttributes.Shared.MultiprocessorCount)
				klog.Infof("\tShared MultiprocessorCountRatio: %f\n", device.MultiprocessorCountRatio)
			}
		}
	}
	return
}
