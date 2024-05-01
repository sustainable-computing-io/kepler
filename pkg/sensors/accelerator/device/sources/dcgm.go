//go:build dcgm
// +build dcgm

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

package sources

import (
	"errors"
	"fmt"
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	_dev "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
	"k8s.io/klog/v2"
)

const (
	debugLevel     = 5
	isSocket       = "0"
	maxMIGProfiles = 15 // use a large number since the profile ids are not linear
	dcgmDevice     = "dcgm"
	dcgmHwType     = "gpu"
)

var (
	dcgmAccImpl               = GPUDcgm{}
	deviceFields []dcgm.Short = []dcgm.Short{
		// https://docs.nvidia.com/datacenter/dcgm/1.7/dcgm-api/group__dcgmFieldIdentifiers.htm
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
	}
	ratioFields  uint = dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE // this is the field that we will use to calculate the utilization per @yuezhu1
	profileInfos map[int]nvml.GpuInstanceProfileInfo
)

type GPUDcgm struct {
	collectionSupported bool
	devices             map[int]_dev.GPUDevice
	migDevices          map[int]map[int]_dev.GPUDevice // list of mig devices for each GPU instance
	libInited           bool
	deviceGroupName     string
	deviceGroupHandle   dcgm.GroupHandle
	fieldGroupName      string
	fieldGroupHandle    dcgm.FieldHandle
	cleanup             func()
}

func init() {
	if _, err := dcgm.Init(dcgm.Standalone, config.DCGMHostEngineEndpoint, isSocket); err != nil {
		klog.Errorf("Error initializing dcgm: %v", err)
		return
	}
	klog.Info("Initializing dcgm Successful")
	_dev.AddDeviceInterface(dcgmDevice, dcgmHwType, dcgmDeviceStartup)
}

func dcgmDeviceStartup() (_dev.AcceleratorInterface, error) {
	a := dcgmAccImpl

	if err := a.InitLib(); err != nil {
		klog.Errorf("Error initializing %s: %v", dcgmDevice, err)
		return nil, err
	}

	if err := a.Init(); err != nil {
		klog.Errorf("failed to StartupDevice: %v", err)
		return nil, err
	}

	klog.Infof("Using %s to obtain gpu power", dcgmDevice)

	return &a, nil
}

func (d *GPUDcgm) Init() error {
	if !d.libInited {
		if err := d.InitLib(); err != nil {
			klog.Errorf("failed to init lib: %v", err)
			return err
		}
	}
	if err := d.createDeviceGroup(); err != nil {
		klog.Errorf("failed to create device group: %v", err)
		d.Shutdown()
		return err
	}

	if err := d.addDevicesToGroup(); err != nil {
		klog.Errorf("failed to add devices to group: %v", err)
		d.Shutdown()
		return err
	}

	if err := d.createFieldGroup(); err != nil {
		klog.Errorf("failed to create field group: %v", err)
		d.Shutdown()
		return err
	}

	if err := d.setupWatcher(); err != nil {
		klog.Errorf("failed to set up watcher: %v", err)
		d.Shutdown()
		return err
	}
	klog.Infof("DCGM initialized successfully")
	d.collectionSupported = true
	return nil
}

func (d *GPUDcgm) InitLib() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not init dcgm: %v", r)
		}
	}()
	cleanup, err := dcgm.Init(dcgm.Standalone, config.DCGMHostEngineEndpoint, isSocket)
	if err != nil {
		klog.Infof("There is no DCGM daemon running in the host: %s", err)
		// embedded mode is not recommended for production per https://github.com/NVIDIA/dcgm-exporter/issues/22#issuecomment-1321521995
		cleanup, err = dcgm.Init(dcgm.Embedded)
		if err != nil {
			klog.Errorf("Could not start DCGM. Error: %s", err)
			if cleanup != nil {
				cleanup()
			}
			return fmt.Errorf("not able to connect to DCGM: %s", err)
		}
		klog.Info("Started DCGM in the Embedded mode ")
	}

	d.devices = make(map[int]_dev.GPUDevice)
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
	d.LoadMIGDevices()
	d.libInited = true
	return nil
}

func (d *GPUDcgm) loadDevices() error {
	d.devices = map[int]_dev.GPUDevice{}
	count, err := nvml.DeviceGetCount()
	if err != nvml.SUCCESS {
		return fmt.Errorf("error getting GPUs: %s", nvml.ErrorString(err))
	}
	for gpuID := 0; gpuID < count; gpuID++ {
		nvmlDeviceHandler, err := nvml.DeviceGetHandleByIndex(gpuID)
		if err != nvml.SUCCESS {
			klog.Errorf("failed to get device handle for index %d: %v", gpuID, nvml.ErrorString(err))
			continue
		}
		device := _dev.GPUDevice{
			DeviceHandler: nvmlDeviceHandler,
			ID:            gpuID,
			IsSubdevice:   false,
		}
		d.devices[gpuID] = device
	}
	return nil
}

// LoadMIGDevices dynamically discover the MIG instances of all GPUs
func (d *GPUDcgm) LoadMIGDevices() {
	d.migDevices = map[int]map[int]_dev.GPUDevice{}

	// find all GPUs and the MIG slices if they exist
	hierarchy, err := dcgm.GetGpuInstanceHierarchy()
	if err != nil {
		klog.Errorf("failed to get GPU Instance hierarchy: %v", err)
		d.Shutdown()
		return
	}

	// the bigger MIG profiles that a GPU can have to be used to calculate the SM ratio
	fullGPUProfile := profileInfos[0]

	for _, entity := range hierarchy.EntityList {
		if entity.Entity.EntityGroupId != dcgm.FE_GPU && entity.Entity.EntityGroupId != dcgm.FE_GPU_I {
			continue
		}

		parentGPUID := int(entity.Parent.EntityId)
		parentDevice := d.devices[parentGPUID]

		// init migDevices
		if _, exit := d.migDevices[parentGPUID]; !exit {
			d.migDevices[parentGPUID] = map[int]_dev.GPUDevice{}
		}

		// find MIG device handler
		nvmlDevID := int(entity.Info.NvmlInstanceId) - 1 // nvidia-smi MIG DEV ID
		migDeviceHandler, ret := parentDevice.DeviceHandler.(nvml.Device).GetMigDeviceHandleByIndex(nvmlDevID)
		if ret != nvml.SUCCESS {
			klog.Errorf("failed to get MIG device handler of GPU %d by index %d: %v", parentGPUID, nvmlDevID, nvml.ErrorString(ret))
			break
		}

		// calculate MIG SM ratio
		profileID := int(entity.Info.NvmlMigProfileId)
		profile := profileInfos[profileID]
		ratio := float64(profile.MultiprocessorCount) / float64(fullGPUProfile.MultiprocessorCount)

		// add MIG device
		migNvmlEntityID := int(entity.Entity.EntityId)
		d.migDevices[parentGPUID][migNvmlEntityID] = _dev.GPUDevice{
			DeviceHandler: migDeviceHandler,
			ID:            migNvmlEntityID,
			ParentID:      parentGPUID,
			MIGSMRatio:    ratio,
			IsSubdevice:   true,
		}
	}
}

func (d *GPUDcgm) loadMIGProfiles() {
	if len(d.devices) == 0 {
		klog.Errorln("DCGM has no GPU to monitor")
		return
	}
	profileInfos = map[int]nvml.GpuInstanceProfileInfo{}
	for _, device := range d.devices {
		for i := 0; i < maxMIGProfiles; i++ {
			profileInfo, ret := device.DeviceHandler.(nvml.Device).GetGpuInstanceProfileInfo(i)
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

func (d *GPUDcgm) GetName() string {
	return dcgmDevice
}

func (d *GPUDcgm) GetType() string {
	return dcgmDevice
}

func (d *GPUDcgm) GetHwType() string {
	return dcgmHwType
}

func (d *GPUDcgm) IsDeviceCollectionSupported() bool {
	return d.collectionSupported
}

func (d *GPUDcgm) SetDeviceCollectionSupported(supported bool) {
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

func (d *GPUDcgm) GetAbsEnergyFromDevice() []uint32 {
	gpuEnergy := []uint32{}
	for _, device := range d.devices {
		power, ret := device.DeviceHandler.(nvml.Device).GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.Errorf("failed to get power usage on device %v: %v\n", device, nvml.ErrorString(ret))
			continue
		}
		// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is
		// necessary to calculate the energy consumption for the entire waiting period
		energy := uint32(uint64(power) * config.SamplePeriodSec)
		gpuEnergy = append(gpuEnergy, energy)
	}
	return gpuEnergy
}

func (d *GPUDcgm) GetDevicesByID() map[int]interface{} {
	devices := make(map[int]interface{})
	for id, device := range d.devices {
		devices[id] = device
	}
	return devices
}

func (d *GPUDcgm) GetDevicesByName() map[string]any {
	devices := make(map[string]interface{})
	return devices
}

func (d *GPUDcgm) GetDeviceInstances() map[int]map[int]interface{} {
	// LoadMIGDevices
	d.LoadMIGDevices()

	devInstances := make(map[int]map[int]interface{})

	for gpuID, migDevices := range d.migDevices {
		devInstances[gpuID] = make(map[int]interface{})
		for migID, device := range migDevices {
			devInstances[gpuID][migID] = device
		}
	}
	return devInstances
}

func (d *GPUDcgm) GetDeviceUtilizationStats(device any) (map[any]interface{}, error) {
	ds := make(map[any]interface{}) // Process Accelerator Metrics
	return ds, nil
}

// GetProcessResourceUtilizationPerDevice returns the GPU utilization per process. The gpuID can be a MIG instance or the main GPU
func (d *GPUDcgm) GetProcessResourceUtilizationPerDevice(device any, since time.Duration) (map[uint32]interface{}, error) {
	processAcceleratorMetrics := map[uint32]_dev.GPUProcessUtilizationSample{}
	pam := make(map[uint32]interface{})
	// Check if the device is of type dev.GPUDevice and extract the DeviceHandler

	switch d := device.(type) {
	case _dev.GPUDevice:
		if d.DeviceHandler == nil {
			return pam, nil
		}

		// Get the processes information, but as GetComputeRunningProcesses only show the process memory allocation,
		// the GPU metrics will be used as the process resource utilization.
		processInfo, ret := d.DeviceHandler.(nvml.Device).GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			klog.Errorf("failed to get running processes: %v", nvml.ErrorString(ret))
		}

		for _, p := range processInfo {
			if !d.IsSubdevice {
				gpuUtilization := float64(0)
				vals, err := dcgm.GetLatestValuesForFields(uint(d.ID), deviceFields)
				if err != nil {
					klog.Errorf("failed to get latest values for fields: %v", err)
					return pam, err
				} else {
					for i := range vals {
						val := &vals[i]
						if val.FieldId == ratioFields {
							gpuUtilization = ToFloat64(val, 100)
						}
					}
				}
				processAcceleratorMetrics[p.Pid] = _dev.GPUProcessUtilizationSample{
					Pid:       p.Pid,
					TimeStamp: uint64(time.Now().UnixNano()),
					// TODO: It does not make sense to use the whole GPU utilization since a GPU might have more than one PID
					// FIXME: As in the original NVML code, we should use here the pinfo.SmUtil from GetProcessUtilization()
					ComputeUtil: uint32(gpuUtilization),
				}
				klog.V(debugLevel).Infof("GPU: %d, PID: %d, GPU Utilization (%d): %f\n", d.ID, p.Pid, ratioFields, gpuUtilization)
			} else {
				vals, err := dcgm.EntityGetLatestValues(dcgm.FE_GPU_I, uint(d.ID), deviceFields)
				if err != nil {
					klog.Errorf("failed to get latest values for fields: %v", err)
					return pam, err
				}
				for i := range vals {
					val := &vals[i]
					if val.FieldId == ratioFields {
						migUtilization := ToFloat64(val, 100)
						// ratio of active multiprocessors to total multiprocessors
						// the MIG metrics represent the utilization of the MIG device. We need to normalize the metric to represent the overall GPU utilization
						// FIXME: the MIG device could have multiple processes, such as using MPS, how to split the MIG utilization between the processes?
						gpuUtilization := migUtilization * d.MIGSMRatio
						processAcceleratorMetrics[p.Pid] = _dev.GPUProcessUtilizationSample{
							Pid:         p.Pid,
							TimeStamp:   uint64(time.Now().UnixNano()),
							ComputeUtil: uint32(gpuUtilization),
						}
						klog.V(debugLevel).Infof("ParentGPU: %d, MIG: %d, PID: %d, GPU Utilization (%d): %f\n", d.ParentID, d.ID, p.Pid, ratioFields, gpuUtilization)
					}
				}
			}
		}
		for k, v := range processAcceleratorMetrics {
			pam[k] = v
		}

		return pam, nil
	default:
		klog.Error("expected _dev.GPUDevice but got come other type")
		return pam, errors.New("invalid device type")
	}
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
	for gpuID := range d.devices {
		err := dcgm.AddEntityToGroup(d.deviceGroupHandle, dcgm.FE_GPU, uint(gpuID))
		if err != nil {
			klog.Errorf("failed to add device %d to group %q: %v", gpuID, d.deviceGroupName, err)
		}
		for migID, migDevice := range d.migDevices[gpuID] {
			err := dcgm.AddEntityToGroup(d.deviceGroupHandle, dcgm.FE_GPU_I, uint(migID))
			if err != nil {
				klog.Errorf("failed to add MIG device %d with parent id %d to group %q: %v", migDevice.ParentID, migDevice.ID, d.deviceGroupName, err)
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

// ToFloat64 converts a dcgm.FieldValue_v1 to float64
// The multiplyFactor is used to convert a percentage represented as a float64 to uint32, maintaining precision and scaling it to 100%.
func ToFloat64(value *dcgm.FieldValue_v1, multiplyFactor float64) float64 {
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
		klog.Errorf("DCGM metric type %v not supported: %v\n", value.FieldType, value)
		return defaultValue
	}
}

func nvmlErrorString(errno nvml.Return) string {
	switch errno {
	case nvml.SUCCESS:
		return "SUCCESS"
	case nvml.ERROR_LIBRARY_NOT_FOUND:
		return "ERROR_LIBRARY_NOT_FOUND"
	}
	return fmt.Sprintf("Error %d", errno)
}
