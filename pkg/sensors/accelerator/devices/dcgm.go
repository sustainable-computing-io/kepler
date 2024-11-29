//go:build !darwin
// +build !darwin

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

package devices

import (
	"errors"
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
	dcgmHwType     = config.GPU
)

var (
	dcgmAccImpl               = gpuDcgm{}
	deviceFields []dcgm.Short = []dcgm.Short{
		// https://docs.nvidia.com/datacenter/dcgm/1.7/dcgm-api/group__dcgmFieldIdentifiers.htm
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
	}
	ratioFields  uint = dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE // this is the field that we will use to calculate the utilization per @yuezhu1
	profileInfos map[int]nvml.GpuInstanceProfileInfo
	dcgmType     DeviceType
)

type gpuDcgm struct {
	collectionSupported bool
	devs                map[int]GPUDevice
	migDevices          map[int]map[int]GPUDevice // list of mig devices for each GPU instance
	libInited           bool
	nvmlInited          bool
	deviceGroupName     string
	deviceGroupHandle   dcgm.GroupHandle
	fieldGroupName      string
	fieldGroupHandle    dcgm.FieldHandle
	cleanup             func()
}

func dcgmCheck(r *Registry) {
	if _, err := dcgm.Init(dcgm.Standalone, config.DCGMHostEngineEndpoint(), isSocket); err != nil {
		klog.V(5).Infof("Error initializing dcgm: %v", err)
		return
	}
	klog.Info("Initializing dcgm Successful")
	dcgmType = DCGM
	if err := addDeviceInterface(r, dcgmType, dcgmHwType, dcgmDeviceStartup); err == nil {
		klog.Infof("Using %s to obtain processor power", dcgmAccImpl.Name())
	} else {
		klog.V(5).Infof("Error registering DCGM: %v", err)
	}
}

func dcgmDeviceStartup() Device {
	a := dcgmAccImpl

	if err := a.InitLib(); err != nil {
		klog.Errorf("Error initializing %s: %v", dcgmType.String(), err)
		return nil
	}

	if err := a.Init(); err != nil {
		klog.Errorf("failed to StartupDevice: %v", err)
		return nil
	}

	klog.Infof("Using %s to obtain gpu power", dcgmType.String())

	return &a
}

func (d *gpuDcgm) Init() error {
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

func (d *gpuDcgm) InitLib() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not init dcgm: %v", r)
		}
	}()
	cleanup, err := dcgm.Init(dcgm.Standalone, config.DCGMHostEngineEndpoint(), isSocket)
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
	d.nvmlInited = false
	d.devs = make(map[int]GPUDevice)
	d.cleanup = cleanup
	dcgm.FieldsInit()

	if err := d.initNVML(); err != nil {
		klog.Errorf("Could not init NVML. Error: %s", err)
		d.Shutdown()
		return err
	}
	d.nvmlInited = true
	if err := d.loadDevices(); err != nil {
		klog.Errorf("Could not load Devices. Error: %s", err)
		d.Shutdown()
		return err
	}
	// after discoverying the devices, load the available MIG profiles
	d.loadMIGProfiles()
	d.LoadMIGDevices()
	d.libInited = true
	return nil
}

func (d *gpuDcgm) loadDevices() error {
	d.devs = map[int]GPUDevice{}
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
		dev := GPUDevice{
			DeviceHandler: nvmlDeviceHandler,
			ID:            gpuID,
			IsSubdevice:   false,
		}
		d.devs[gpuID] = dev
	}
	return nil
}

// LoadMIGDevices dynamically discover the MIG instances of all GPUs
func (d *gpuDcgm) LoadMIGDevices() {
	d.migDevices = map[int]map[int]GPUDevice{}

	// find all GPUs and the MIG slices if they exist
	hierarchy, err := dcgm.GetGpuInstanceHierarchy()
	if err != nil {
		klog.Errorf("failed to get GPU Instance hierarchy: %v", err)
		d.Shutdown()
		return
	}

	// the bigger MIG profiles that a GPU can have to be used to calculate the SM ratio
	fullGPUProfile := profileInfos[0]

	for i := range hierarchy.EntityList {
		entity := &hierarchy.EntityList[i]
		if entity.Entity.EntityGroupId != dcgm.FE_GPU && entity.Entity.EntityGroupId != dcgm.FE_GPU_I {
			continue
		}

		parentGPUID := int(entity.Parent.EntityId)
		parentDevice := d.devs[parentGPUID]

		// init migDevices
		if _, exit := d.migDevices[parentGPUID]; !exit {
			d.migDevices[parentGPUID] = map[int]GPUDevice{}
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
		d.migDevices[parentGPUID][migNvmlEntityID] = GPUDevice{
			DeviceHandler: migDeviceHandler,
			ID:            migNvmlEntityID,
			ParentID:      parentGPUID,
			MIGSMRatio:    ratio,
			IsSubdevice:   true,
		}
	}
}

func (d *gpuDcgm) loadMIGProfiles() {
	if len(d.devs) == 0 {
		klog.Errorln("DCGM has no GPU to monitor")
		return
	}
	profileInfos = map[int]nvml.GpuInstanceProfileInfo{}
	for _, dev := range d.devs {
		for i := 0; i < maxMIGProfiles; i++ {
			profileInfo, ret := dev.DeviceHandler.(nvml.Device).GetGpuInstanceProfileInfo(i)
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

func (d *gpuDcgm) Name() string {
	return dcgmType.String()
}

func (d *gpuDcgm) DevType() DeviceType {
	return dcgmType
}

func (d *gpuDcgm) HwType() string {
	return dcgmHwType
}

func (d *gpuDcgm) IsDeviceCollectionSupported() bool {
	return d.collectionSupported
}

func (d *gpuDcgm) SetDeviceCollectionSupported(supported bool) {
	d.collectionSupported = supported
}

func (d *gpuDcgm) Shutdown() bool {
	if d.nvmlInited {
		nvml.Shutdown()
	}
	dcgm.FieldsTerm()
	if d.deviceGroupName != "" {
		if err := dcgm.DestroyGroup(d.deviceGroupHandle); err != nil {
			klog.Errorf("failed to destroy group %v", err)
		}
	}
	if d.fieldGroupName != "" {
		if err := dcgm.FieldGroupDestroy(d.fieldGroupHandle); err != nil {
			klog.Errorf("failed to destroy field group %v", err)
		}
	}
	if d.cleanup != nil {
		d.cleanup()
	}
	d.collectionSupported = false
	d.libInited = false
	d.nvmlInited = false
	return true
}

func (d *gpuDcgm) AbsEnergyFromDevice() []uint32 {
	gpuEnergy := []uint32{}
	for _, dev := range d.devs {
		power, ret := dev.DeviceHandler.(nvml.Device).GetPowerUsage()
		if ret != nvml.SUCCESS {
			klog.Errorf("failed to get power usage on device %v: %v\n", dev, nvml.ErrorString(ret))
			continue
		}
		// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is
		// necessary to calculate the energy consumption for the entire waiting period
		energy := uint32(uint64(power) * config.SamplePeriodSec())
		gpuEnergy = append(gpuEnergy, energy)
	}
	return gpuEnergy
}

func (d *gpuDcgm) DevicesByID() map[int]any {
	devs := make(map[int]any)
	for id, dev := range d.devs {
		devs[id] = dev
	}
	return devs
}

func (d *gpuDcgm) DevicesByName() map[string]any {
	devs := make(map[string]any)
	return devs
}

func (d *gpuDcgm) DeviceInstances() map[int]map[int]any {
	// LoadMIGDevices
	d.LoadMIGDevices()

	devInstances := make(map[int]map[int]any)

	for gpuID, migDevices := range d.migDevices {
		devInstances[gpuID] = make(map[int]any)
		for migID, dev := range migDevices {
			devInstances[gpuID][migID] = dev
		}
	}
	return devInstances
}

func (d *gpuDcgm) DeviceUtilizationStats(dev any) (map[any]any, error) {
	ds := make(map[any]any) // Process Accelerator Metrics
	return ds, nil
}

// ProcessResourceUtilizationPerDevice returns the GPU utilization per process. The gpuID can be a MIG instance or the main GPU
func (d *gpuDcgm) ProcessResourceUtilizationPerDevice(dev any, since time.Duration) (map[uint32]any, error) {
	processAcceleratorMetrics := map[uint32]GPUProcessUtilizationSample{}
	pam := make(map[uint32]any)
	// Check if the device is of type dev.GPUDevice and extract the DeviceHandler

	switch d := dev.(type) {
	case GPUDevice:
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
				processAcceleratorMetrics[p.Pid] = GPUProcessUtilizationSample{
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
						// the MIG metrics represent the utilization of the MIG  We need to normalize the metric to represent the overall GPU utilization
						// FIXME: the MIG device could have multiple processes, such as using MPS, how to split the MIG utilization between the processes?
						gpuUtilization := migUtilization * d.MIGSMRatio
						processAcceleratorMetrics[p.Pid] = GPUProcessUtilizationSample{
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
		klog.Error("expected GPUDevice but got come other type")
		return pam, errors.New("invalid device type")
	}
}

// helper functions
func (d *gpuDcgm) initNVML() error {
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		d.collectionSupported = false
		d.Shutdown()
		return fmt.Errorf("failed to init nvml. %s", nvmlErrorString(ret))
	}
	return nil
}

func (d *gpuDcgm) createDeviceGroup() error {
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

func (d *gpuDcgm) addDevicesToGroup() error {
	for gpuID := range d.devs {
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

func (d *gpuDcgm) createFieldGroup() error {
	fieldGroupName := "fld-grp-" + time.Now().Format("2006-01-02-15-04-05")
	fieldGroup, err := dcgm.FieldGroupCreate(fieldGroupName, deviceFields)
	if err != nil {
		return fmt.Errorf("failed to create field group %q: %v", fieldGroupName, err)
	}
	d.fieldGroupName = fieldGroupName
	d.fieldGroupHandle = fieldGroup
	return nil
}

func (d *gpuDcgm) setupWatcher() error {
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
