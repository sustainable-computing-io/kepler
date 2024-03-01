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
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"k8s.io/klog/v2"
)

const (
	debugLevel     = 5
	isSocket       = "0"
	maxMIGProfiles = 15 // use a large number since the profile ids are not linear
)

var (
	deviceFields []dcgm.Short = []dcgm.Short{
		// https://docs.nvidia.com/datacenter/dcgm/1.7/dcgm-api/group__dcgmFieldIdentifiers.htm
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
	}
	deviceFieldsString = []string{
		"dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE",
	}
	ratioField uint = dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE // this is the field that we will use to calculate the utilization per @yuezhu1
	// gpuMigArray              [][]MigDevice
	// totalMultiProcessorCount map[string]int
	profileInfos map[int]nvml.GpuInstanceProfileInfo
)

// MigDeviceInfo holds information about a MIG device
type MigDeviceInfo struct {
	Entity                   dcgm.GroupEntityPair
	Parent                   dcgm.GroupEntityPair
	Info                     dcgm.MigEntityInfo
	Profile                  nvml.GpuInstanceProfileInfo
	TotalMultiprocessorCount float64
}

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
	entities            map[int]map[int]MigDeviceInfo
	cleanup             func()
}

func (d *GPUDcgm) GetName() string {
	return "dcgm"
}

func (d *GPUDcgm) InitLib() error {
	d.devices = make(map[string]interface{})
	d.entities = make(map[int]map[int]MigDeviceInfo)

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
	if err := d.loadDevices(); err != nil {
		d.Shutdown()
		return err
	}
	// after discoverying the devices, load the available MIG profiles
	d.loadMIGProfiles()
	d.UpdateMIGEntityList()
	d.libInited = true
	return nil
}

func (d *GPUDcgm) loadDevices() error {
	count, err := nvml.DeviceGetCount()
	if err != nvml.SUCCESS {
		return fmt.Errorf("Error getting GPUs: %s", nvml.ErrorString(err))
	}
	for gpuID := 0; gpuID < count; gpuID++ {
		device, err := nvml.DeviceGetHandleByIndex(gpuID)
		if err != nvml.SUCCESS {
			klog.Errorf("failed to get device handle for index %d: %v", gpuID, nvml.ErrorString(err))
			continue
		}
		d.devices[fmt.Sprintf("%v", gpuID)] = device
	}
	return nil
}

func (d *GPUDcgm) loadMIGProfiles() {
	if len(d.devices) == 0 {
		klog.Errorln("CDGM has no GPU to monitor")
		return
	}
	profileInfos = map[int]nvml.GpuInstanceProfileInfo{}
	for _, device := range d.devices {
		for i := 0; i < maxMIGProfiles; i++ {
			profileInfo, ret := device.(nvml.Device).GetGpuInstanceProfileInfo(i)
			if ret != nvml.SUCCESS {
				continue
			}
			id := int(profileInfo.Id)
			if _, exit := profileInfos[id]; !exit {
				profileInfos[id] = profileInfo
			}
		}
	}
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

	// as the GetAbsEnergyFromGPU is called only once and GetProcessResourceUtilizationPerDevice is called for each GPU, it is better to update the MIG list here
	d.UpdateMIGEntityList()
	return gpuEnergy
}

func (d *GPUDcgm) GetGpus() map[string]interface{} {
	return d.devices
}

// GetProcessResourceUtilizationPerDevice returns the GPU utilization per process. The gpuID can be a MIG instance or the main GPU
func (d *GPUDcgm) GetProcessResourceUtilizationPerDevice(device interface{}, gpuID int, since time.Duration) (map[uint32]ProcessUtilizationSample, error) {
	processAcceleratorMetrics := map[uint32]ProcessUtilizationSample{}
	if device == nil {
		return processAcceleratorMetrics, nil
	}

	var vals []dcgm.FieldValue_v1
	var isMIG bool
	var ret nvml.Return
	var err error
	multiprocessorCountRatio := 1.0 // ratio used to normalize the mig metric to the entire GPU

	if isMIG, ret = device.(nvml.Device).IsMigDeviceHandle(); ret == nvml.SUCCESS && isMIG {
		migID, _ := device.(nvml.Device).GetIndex()
		migInfo := d.entities[gpuID][migID]

		klog.V(debugLevel).Infof("MIG Device gpuID %v, migID %v\n", gpuID, migID)
		klog.V(debugLevel).Infof("MIG Device %v\n", migInfo.Entity.EntityId)
		vals, err = dcgm.EntityGetLatestValues(dcgm.FE_GPU_I, migInfo.Entity.EntityId, deviceFields)
		if err != nil {
			klog.V(debugLevel).Infof("failed to get latest values for fields: %v", err)
			return processAcceleratorMetrics, err
		}
		multiprocessorCountRatio = float64(migInfo.Profile.MultiprocessorCount) / migInfo.TotalMultiprocessorCount
	} else {
		if ret != nvml.SUCCESS {
			klog.V(debugLevel).Infof("failed to get GPU ID: %v", err)
			return processAcceleratorMetrics, err
		}
		klog.V(debugLevel).Infof("Device %v\n", gpuID)
		vals, err = dcgm.GetLatestValuesForFields(uint(gpuID), deviceFields)
		if err != nil {
			klog.V(debugLevel).Infof("failed to get latest values for fields: %v", err)
			return processAcceleratorMetrics, err
		}
	}

	processInfo, ret := device.(nvml.Device).GetComputeRunningProcesses()
	if ret != nvml.SUCCESS {
		klog.V(debugLevel).Infof("failed to get running processes: %v", nvml.ErrorString(ret))
	}
	for _, p := range processInfo {
		klog.V(debugLevel).Infof("pid: %d, memUtil: %d gpu instance id %d compute id %d\n", p.Pid, p.UsedGpuMemory, p.GpuInstanceId, p.ComputeInstanceId)
		if isMIG { // this is a MIG, get it entity id and reads the related fields
			for _, val := range vals {
				if val.FieldId == ratioField {
					migUtilization := ToFloat64(val, 100)
					// ratio of active multiprocessors to total multiprocessors
					// the MIG metrics represent the utilization of the MIG device. We need to normalize the metric to represent the overall GPU utilization
					// FIXME: the MIG device could have multiple processes, how to split the MIG utilization between the processes?
					normalizedComputeUtil := migUtilization * multiprocessorCountRatio
					klog.V(debugLevel).Infof("pid %d computeUtil %f multiprocessor count ratio %v\n", p.Pid, normalizedComputeUtil, multiprocessorCountRatio)
					processAcceleratorMetrics[p.Pid] = ProcessUtilizationSample{
						Pid:         p.Pid,
						TimeStamp:   uint64(time.Now().UnixNano()),
						ComputeUtil: uint32(normalizedComputeUtil),
					}
				}
			}
			klog.V(debugLevel).Infof("\n")
		} else {
			gpuUtilization := float64(0)
			if err == nil {
				for _, val := range vals {
					if val.FieldId == ratioField {
						gpuUtilization = ToFloat64(val, 100)
					}
				}
			}
			processAcceleratorMetrics[p.Pid] = ProcessUtilizationSample{
				Pid:       p.Pid,
				TimeStamp: uint64(time.Now().UnixNano()),
				// TODO: It does not make sense to use the whole GPU utilization since a GPU might have more than one PID
				// FIXME: As in the original NVML code, we should use here the pinfo.SmUtil from GetProcessUtilization()
				ComputeUtil: uint32(gpuUtilization),
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
	for gpuID, infos := range d.entities {
		// Add GPUs
		err := dcgm.AddEntityToGroup(d.deviceGroupHandle, dcgm.FE_GPU, uint(gpuID))
		if err != nil {
			klog.Infof("failed to add device %d to group %q: %v", gpuID, d.deviceGroupName, err)
		}
		// Add MIGs
		for _, info := range infos {
			err := dcgm.AddEntityToGroup(d.deviceGroupHandle, dcgm.FE_GPU_I, info.Entity.EntityId)
			if err != nil {
				klog.Infof("failed to add MIG device %d/%d to group %q: %v", gpuID, info.Entity.EntityId, d.deviceGroupName, err)
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

// UpdateMIGEntityList dynamically discover the MIG instances of all GPUs
func (d *GPUDcgm) UpdateMIGEntityList() {
	hierarchy, err := dcgm.GetGpuInstanceHierarchy()
	if err != nil {
		d.Shutdown()
		return
	}
	d.entities = map[int]map[int]MigDeviceInfo{}
	fullGPUProfile := profileInfos[0]

	for _, entity := range hierarchy.EntityList {
		if entity.Entity.EntityGroupId != dcgm.FE_GPU && entity.Entity.EntityGroupId != dcgm.FE_GPU_I {
			continue
		}
		profileID := int(entity.Info.NvmlMigProfileId)
		profile := profileInfos[profileID]
		nvmlGpuIndex := int(entity.Info.NvmlGpuIndex)
		nvmlInstanceId := int(entity.Info.NvmlInstanceId) - 1 // this index is the same index used in GetMigDeviceHandleByIndex
		if _, exist := d.entities[nvmlGpuIndex]; !exist {
			d.entities[nvmlGpuIndex] = map[int]MigDeviceInfo{}
		}
		if _, exist := d.entities[nvmlGpuIndex][nvmlInstanceId]; !exist {
			mig := MigDeviceInfo{}
			mig.Entity = entity.Entity
			mig.Parent = entity.Parent
			mig.Info = entity.Info
			mig.Profile = profile
			mig.TotalMultiprocessorCount = float64(fullGPUProfile.MultiprocessorCount)
			d.entities[nvmlGpuIndex][nvmlInstanceId] = mig
		}
	}
}

// ToFloat64 converts a dcgm.FieldValue_v1 to float64
// The multiplyFactor is used to convert a percentage represented as a float64 to uint32, maintaining precision and scaling it to 100%.
func ToFloat64(value dcgm.FieldValue_v1, multiplyFactor float64) float64 {
	defaultValue := float64(0)
	switch v := value.FieldType; v {

	// Floating-point
	case dcgm.DCGM_FT_DOUBLE:
		switch v := value.Float64(); v {
		case dcgm.DCGM_FT_FP64_BLANK:
			return defaultValue
		case dcgm.DCGM_FT_FP64_NOT_FOUND:
			return defaultValue
		case dcgm.DCGM_FT_FP64_NOT_SUPPORTED:
			return defaultValue
		case dcgm.DCGM_FT_FP64_NOT_PERMISSIONED:
			return defaultValue
		default:
			return v * multiplyFactor
		}

	// Int32 and Int64
	case dcgm.DCGM_FT_INT64:
		switch v := value.Int64(); v {
		case dcgm.DCGM_FT_INT32_BLANK:
			return defaultValue
		case dcgm.DCGM_FT_INT32_NOT_FOUND:
			return defaultValue
		case dcgm.DCGM_FT_INT32_NOT_SUPPORTED:
			return defaultValue
		case dcgm.DCGM_FT_INT32_NOT_PERMISSIONED:
			return defaultValue
		case dcgm.DCGM_FT_INT64_BLANK:
			return defaultValue
		case dcgm.DCGM_FT_INT64_NOT_FOUND:
			return defaultValue
		case dcgm.DCGM_FT_INT64_NOT_SUPPORTED:
			return defaultValue
		case dcgm.DCGM_FT_INT64_NOT_PERMISSIONED:
			return defaultValue
		default:
			return float64(v) * multiplyFactor
		}

	default:
		klog.Errorf("DCGM metric type %s not supported: %v\n", value.FieldType, value)
		return defaultValue
	}
}
