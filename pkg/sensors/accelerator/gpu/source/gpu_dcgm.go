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
	"fmt"
	"strconv"
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"k8s.io/klog/v2"
)

const (
	debugLevel = 5
	isSocket   = "0"
)

var (
	deviceFields []dcgm.Short = []dcgm.Short{
		// https://docs.nvidia.com/datacenter/dcgm/1.7/dcgm-api/group__dcgmFieldIdentifiers.htm
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
	}
	deviceFieldsString = []string{
		"dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE",
	}
	ratioFields              uint = dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE // this is the field that we will use to calculate the utilization per @yuezhu1
	SkipDCGMValue                 = "SKIPPING DCGM VALUE"
	FailedToConvert               = "ERROR - FAILED TO CONVERT TO STRING"
	gpuMigArray              [][]MigDevice
	totalMultiProcessorCount map[string]int
)

type GPUDcgm struct {
	collectionSupported bool
	libInited           bool
	devices             map[string]interface{}
	deviceGroupName     string
	deviceGroupHandle   dcgm.GroupHandle
	fieldGroupName      string
	fieldGroupHandle    dcgm.FieldHandle
	pidGroupName        string
	pidGroupHandle      dcgm.GroupHandle // TODO: wait till https://github.com/NVIDIA/go-dcgm/issues/59 is resolved
	entities            map[string]dcgm.GroupEntityPair
	cleanup             func()
}

func (d *GPUDcgm) GetName() string {
	return "dcgm"
}

func (d *GPUDcgm) InitLib() error {
	d.devices = make(map[string]interface{})
	d.entities = make(map[string]dcgm.GroupEntityPair)

	cleanup, err := dcgm.Init(dcgm.Standalone, config.DCGMHostEngineEndpoint, isSocket)
	if err != nil {
		klog.Warningf("There is no DCGM daemon running in the host: %s", err)
		// embeded mode is not recommended for production per https://github.com/NVIDIA/dcgm-exporter/issues/22#issuecomment-1321521995
		cleanup, err = dcgm.Init(dcgm.Embedded)
		if err != nil {
			klog.Warningf("Could not start DCGM. Error: %s", err)
			if cleanup != nil {
				cleanup()
			}
			return fmt.Errorf("not able to connect to DCGM: %s", err)
		}
		klog.V(1).Infof("Started DCGM in the Embedded mode", err)
	}
	d.cleanup = cleanup
	dcgm.FieldsInit()

	if err := d.initNVML(); err != nil {
		d.Shutdown()
		return err
	}
	d.libInited = true
	return nil
}

func (d *GPUDcgm) Init() error {
	if !d.libInited {
		if err := d.InitLib(); err != nil {
			klog.Infof("failed to init lib: %v", err)
			return err
		}
	}
	if err := d.createDeviceGroup(); err != nil {
		klog.Infof("failed to create device group: %v", err)
		d.Shutdown()
		return err
	}

	if err := d.addDevicesToGroup(); err != nil {
		klog.Infof("failed to add devices to group: %v", err)
		d.Shutdown()
		return err
	}

	if err := d.createFieldGroup(); err != nil {
		klog.Infof("failed to create field group: %v", err)
		d.Shutdown()
		return err
	}

	if err := d.setupWatcher(); err != nil {
		klog.Infof("failed to set up watcher: %v", err)
		d.Shutdown()
		return err
	}
	klog.Infof("DCGM initialized successfully")
	d.collectionSupported = true
	return nil
}

func (d *GPUDcgm) IsGPUCollectionSupported() bool {
	return d.collectionSupported
}

func (d *GPUDcgm) SetGPUCollectionSupported(supported bool) {
	d.collectionSupported = supported
}

func (d *GPUDcgm) Shutdown() bool {
	nvml.Shutdown()
	dcgm.FieldsTerm()
	if d.deviceGroupName != "" {
		dcgm.DestroyGroup(d.deviceGroupHandle)
	}
	if d.fieldGroupName != "" {
		dcgm.FieldGroupDestroy(d.fieldGroupHandle)
	}
	if d.cleanup != nil {
		d.cleanup()
	}
	d.collectionSupported = false
	d.libInited = false
	return true
}

func (d *GPUDcgm) GetAbsEnergyFromGPU() []uint32 {
	gpuEnergy := []uint32{}
	for _, device := range d.devices {
		power, ret := device.(nvml.Device).GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.V(2).Infof("failed to get power usage on device %v: %v\n", device, nvml.ErrorString(ret))
			continue
		}
		// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is
		// necessary to calculate the energy consumption for the entire waiting period
		energy := uint32(uint64(power) * config.SamplePeriodSec)
		gpuEnergy = append(gpuEnergy, energy)
	}
	return gpuEnergy
}

func (d *GPUDcgm) GetGpus() map[string]interface{} {
	return d.devices
}

func (d *GPUDcgm) GetProcessResourceUtilizationPerDevice(device interface{}, deviceName string, since time.Duration) (map[uint32]ProcessUtilizationSample, error) {
	processAcceleratorMetrics := map[uint32]ProcessUtilizationSample{}

	if device == nil { // this is a MIG device, it is already tracked in the parent device
		return processAcceleratorMetrics, nil
	}

	var vals, miVals []dcgm.FieldValue_v1
	var err error

	klog.V(debugLevel).Infof("Device %v\n", deviceName)

	deviceIndex, strErr := strconv.Atoi(deviceName)
	if strErr != nil {
		klog.V(debugLevel).Infof("failed to convert %q to an integer: %v", deviceName, strErr)
		return processAcceleratorMetrics, strErr
	}
	vals, err = dcgm.GetLatestValuesForFields(uint(deviceIndex), deviceFields)
	if err != nil {
		klog.V(debugLevel).Infof("failed to get latest values for fields: %v", err)
		return processAcceleratorMetrics, err
	}
	gpuSMActive := uint32(0)
	if err == nil {
		for i, val := range vals {
			value := ToString(val)
			label := deviceFieldsString[i]
			if val.FieldId == ratioFields {
				computeUtil, _ := strconv.ParseFloat(value, 32)
				gpuSMActive = uint32(computeUtil * 100)
			}
			klog.V(debugLevel).Infof("Device %v Label %v Val: %v", deviceName, label, ToString(val))
		}
		klog.V(debugLevel).Infof("\n")
	}
	processInfo, ret := device.(nvml.Device).GetComputeRunningProcesses()
	if ret != nvml.SUCCESS {
		klog.V(debugLevel).Infof("failed to get running processes: %v", nvml.ErrorString(ret))
		return processAcceleratorMetrics, fmt.Errorf("failed to get running processes: %v", nvml.ErrorString(ret))
	}
	for _, p := range processInfo {
		klog.V(debugLevel).Infof("pid: %d, memUtil: %d gpu instance id %d compute id %d\n", p.Pid, p.UsedGpuMemory, p.GpuInstanceId, p.ComputeInstanceId)
		if p.GpuInstanceId > 0 && p.GpuInstanceId < uint32(len(gpuMigArray[deviceIndex])) { // this is a MIG, get it entity id and reads the related fields
			entityName := gpuMigArray[deviceIndex][p.GpuInstanceId].EntityName
			multiprocessorCountRatio := gpuMigArray[deviceIndex][p.GpuInstanceId].MultiprocessorCountRatio
			mi := d.entities[entityName]
			miVals, err = dcgm.EntityGetLatestValues(mi.EntityGroupId, mi.EntityId, deviceFields)
			if err == nil {
				for i, val := range miVals {
					label := deviceFieldsString[i]
					value := ToString(val)
					klog.V(debugLevel).Infof("Device %v Label %v Val: %v", entityName, label, value)
					if val.FieldId == ratioFields {
						floatVal, _ := strconv.ParseFloat(value, 32)
						// ratio of active multiprocessors to total multiprocessors
						computeUtil := uint32(floatVal * 100 * multiprocessorCountRatio)
						klog.V(debugLevel).Infof("pid %d computeUtil %d multiprocessor count ratio %v\n", p.Pid, computeUtil, multiprocessorCountRatio)
						processAcceleratorMetrics[p.Pid] = ProcessUtilizationSample{
							Pid:         p.Pid,
							TimeStamp:   uint64(time.Now().UnixNano()),
							ComputeUtil: computeUtil,
						}
					}
				}
				klog.V(debugLevel).Infof("\n")
			}
		} else {
			processAcceleratorMetrics[p.Pid] = ProcessUtilizationSample{
				Pid:         p.Pid,
				TimeStamp:   uint64(time.Now().UnixNano()),
				ComputeUtil: gpuSMActive, // if this is not a MIG, we will use the GPU SM active value. FIXME: what if there are multiple pids in the same GPU?
			}
		}
	}

	return processAcceleratorMetrics, nil
}

// helper functions
func (d *GPUDcgm) initNVML() error {
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		d.collectionSupported = false
		d.Shutdown()
		return fmt.Errorf("failed to init nvml. %s", nvmlErrorString(ret))
	}
	return nil
}

func (d *GPUDcgm) createDeviceGroup() error {
	deviceGroupName := "dev-grp-" + time.Now().Format("2006-01-02-15-04-05")
	deviceGroup, err := dcgm.CreateGroup(deviceGroupName)
	if err != nil {
		return fmt.Errorf("failed to create group %q: %v", deviceGroupName, err)
	}
	d.deviceGroupName = deviceGroupName
	d.deviceGroupHandle = deviceGroup
	klog.Infof("Created device group %q", deviceGroupName)
	return nil
}

func (d *GPUDcgm) addDevicesToGroup() error {
	supportedDeviceIndices, err := dcgm.GetSupportedDevices()
	if err != nil {
		return fmt.Errorf("failed to find supported devices: %v", err)
	}
	klog.V(debugLevel).Infof("found %d supported devices", len(supportedDeviceIndices))
	for _, gpuIndex := range supportedDeviceIndices {
		err = dcgm.AddEntityToGroup(d.deviceGroupHandle, dcgm.FE_GPU, gpuIndex)
		if err != nil {
			klog.Infof("failed to add device %d to group %q: %v", gpuIndex, d.deviceGroupName, err)
		} else {
			device, ret := nvml.DeviceGetHandleByIndex(int(gpuIndex))
			if ret != nvml.SUCCESS {
				klog.Infof("failed to get nvml device %d: %v ", gpuIndex, nvml.ErrorString(ret))
				continue
			}
			d.devices[fmt.Sprintf("%v", gpuIndex)] = device
			d.entities[fmt.Sprintf("%v", gpuIndex)] = dcgm.GroupEntityPair{dcgm.FE_GPU, gpuIndex}
		}
	}

	// add entity to the group
	hierarchy, err := dcgm.GetGpuInstanceHierarchy()
	if err != nil {
		d.Shutdown()
		return fmt.Errorf("failed to get gpu hierachy: %v", err)
	}

	if hierarchy.Count > 0 {
		// if MIG is enabled, we need to know the hierarchy as well as the multiprocessor count in each device.
		// we will use the multiprocessor count to calculate the utilization of each instance
		if gpuMigArray, totalMultiProcessorCount, err = RetriveFromNvidiaSMI(false); err != nil {
			klog.Infof("failed to retrive from nvidia-smi: %v", err)
			// if we cannot get the multiprocessor count, we will not be able to calculate the utilization
		}
		for i := uint(0); i < hierarchy.Count; i++ {
			if hierarchy.EntityList[i].Parent.EntityGroupId == dcgm.FE_GPU {
				// add a GPU instance
				info := hierarchy.EntityList[i].Info
				entityId := hierarchy.EntityList[i].Entity.EntityId
				gpuId := hierarchy.EntityList[i].Parent.EntityId
				klog.V(debugLevel).Infof("gpu id %v entity id %v gpu index %v instance id %v", gpuId, entityId, info.NvmlGpuIndex, info.NvmlInstanceId)
				entityName := fmt.Sprintf("entity-%d", entityId)
				gpuMigArray[info.NvmlGpuIndex][info.NvmlInstanceId].EntityName = entityName
				err = dcgm.AddEntityToGroup(d.deviceGroupHandle, dcgm.FE_GPU_I, entityId)
				d.entities[entityName] = dcgm.GroupEntityPair{dcgm.FE_GPU_I, entityId}
				klog.V(debugLevel).Infof("Adding GPU instance %d, err: %v", entityId, err)
			}
		}
	}
	return nil
}

func (d *GPUDcgm) createFieldGroup() error {
	fieldGroupName := "fld-grp-" + time.Now().Format("2006-01-02-15-04-05")
	fieldGroup, err := dcgm.FieldGroupCreate(fieldGroupName, deviceFields)
	if err != nil {
		return fmt.Errorf("failed to create field group %q: %v", fieldGroupName, err)
	}
	d.fieldGroupName = fieldGroupName
	d.fieldGroupHandle = fieldGroup
	return nil
}

func (d *GPUDcgm) setupWatcher() error {
	// watch interval has an impact on cpu usage, set it carefully
	err := dcgm.WatchFieldsWithGroupEx(d.fieldGroupHandle, d.deviceGroupHandle, int64(1000)*1000, 0.0, 1)
	if err != nil {
		return fmt.Errorf("failed to set up watcher, err %v", err)
	}
	return nil
}

// ToString converts a dcgm.FieldValue_v1 to a string
// credit to dcgm_exporter
func ToString(value dcgm.FieldValue_v1) string {
	switch v := value.Int64(); v {
	case dcgm.DCGM_FT_INT32_BLANK:
		return SkipDCGMValue
	case dcgm.DCGM_FT_INT32_NOT_FOUND:
		return SkipDCGMValue
	case dcgm.DCGM_FT_INT32_NOT_SUPPORTED:
		return SkipDCGMValue
	case dcgm.DCGM_FT_INT32_NOT_PERMISSIONED:
		return SkipDCGMValue
	case dcgm.DCGM_FT_INT64_BLANK:
		return SkipDCGMValue
	case dcgm.DCGM_FT_INT64_NOT_FOUND:
		return SkipDCGMValue
	case dcgm.DCGM_FT_INT64_NOT_SUPPORTED:
		return SkipDCGMValue
	case dcgm.DCGM_FT_INT64_NOT_PERMISSIONED:
		return SkipDCGMValue
	}
	switch v := value.Float64(); v {
	case dcgm.DCGM_FT_FP64_BLANK:
		return SkipDCGMValue
	case dcgm.DCGM_FT_FP64_NOT_FOUND:
		return SkipDCGMValue
	case dcgm.DCGM_FT_FP64_NOT_SUPPORTED:
		return SkipDCGMValue
	case dcgm.DCGM_FT_FP64_NOT_PERMISSIONED:
		return SkipDCGMValue
	}
	switch v := value.FieldType; v {
	case dcgm.DCGM_FT_STRING:
		return value.String()
	case dcgm.DCGM_FT_DOUBLE:
		return fmt.Sprintf("%f", value.Float64())
	case dcgm.DCGM_FT_INT64:
		return fmt.Sprintf("%d", value.Int64())
	default:
		return FailedToConvert
	}

	return FailedToConvert
}
