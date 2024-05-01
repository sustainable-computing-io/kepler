/*
 * Copyright (c) 2022, HabanaLabs Ltd.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the Lic
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package gohlml is a Go package serves as a bridge to work
// with hlml C library. It allows access to native Habana device
// commands and information.
package gohlml

/*
#cgo habana LDFLAGS: "/usr/lib/habanalabs/libhlml.so" -ldl -Wl,--unresolved-symbols=ignore-all
#include "hlml.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"
)

const (
	szUUID = 256
	// HlmlCriticalError indicates a critical error in the device
	HlmlCriticalError = C.HLML_EVENT_CRITICAL_ERR
	// HLDriverPath indicates on habana device dir
	HLDriverPath = "/sys/class/accel"
	// HLModulePath indicates on habana module dir
	HLModulePath = "/sys/module/habanalabs"
	// BITSPerLong repsenets 64 bits in logs
	BITSPerLong = 64
)

var pciBasePath = "/sys/bus/pci/devices"

// Device struct maps to C HLML structure
type Device struct{ dev C.hlml_device_t }

// EventSet is a cast of the C type of the hlml event set
type EventSet struct{ set C.hlml_event_set_t }

// Event contains uuid and event type
type Event struct {
	Serial string
	Etype  uint64
}

// PCIInfo contains the PCI properties of the device
type PCIInfo struct {
	BusID    string
	DeviceID uint
}

var (
	ErrNotIntialized      = errors.New("hlml not initialized")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotSupported       = errors.New("not supported")
	ErrAlreadyInitialized = errors.New("hlml already initialized")
	ErrNotFound           = errors.New("not found")
	ErrInsufficientSize   = errors.New("insufficient size")
	ErrDriverNotLoaded    = errors.New("driver not loaded")
	ErrAipIsLost          = errors.New("aip is lost")
	ErrMemoryError        = errors.New("memory error")
	ErrNoData             = errors.New("no data")
	ErrUnknownError       = errors.New("unknown error")
)

func errorString(ret C.hlml_return_t) error {
	switch ret {
	case C.HLML_SUCCESS:
		fallthrough
	case C.HLML_ERROR_TIMEOUT:
		return nil
	case C.HLML_ERROR_UNINITIALIZED:
		return ErrNotIntialized
	case C.HLML_ERROR_INVALID_ARGUMENT:
		return ErrInvalidArgument
	case C.HLML_ERROR_NOT_SUPPORTED:
		return ErrNotSupported
	case C.HLML_ERROR_ALREADY_INITIALIZED:
		return ErrAlreadyInitialized
	case C.HLML_ERROR_NOT_FOUND:
		return ErrNotFound
	case C.HLML_ERROR_INSUFFICIENT_SIZE:
		return ErrInsufficientSize
	case C.HLML_ERROR_DRIVER_NOT_LOADED:
		return ErrDriverNotLoaded
	case C.HLML_ERROR_AIP_IS_LOST:
		return ErrAipIsLost
	case C.HLML_ERROR_MEMORY:
		return ErrMemoryError
	case C.HLML_ERROR_NO_DATA:
		return ErrNoData
	case C.HLML_ERROR_UNKNOWN:
		return ErrUnknownError
	}

	return fmt.Errorf("invalid HLML error return code %d", ret)
}

// Initialize initializes the HLML library
func Initialize() error {
	return errorString(C.hlml_init())
}

// InitWithLogs initializes the HLML library with logging on
func InitWithLogs() error {
	return errorString(C.hlml_init_with_flags(0x6))
}

// Shutdown shutdowns the HLML library
func Shutdown() error {
	return errorString(C.hlml_shutdown())
}

// DeviceCount gets number of Habana devices in the system
func DeviceCount() (uint, error) {
	var NumOfDevices C.uint

	rc := C.hlml_device_get_count(&NumOfDevices)
	return uint(NumOfDevices), errorString(rc)
}

// DeviceHandleByIndex gets a handle to a particular device by index
func DeviceHandleByIndex(idx uint) (Device, error) {
	var dev C.hlml_device_t

	rc := C.hlml_device_get_handle_by_index(C.uint(idx), &dev)
	return Device{dev}, errorString(rc)
}

// DeviceHandleByUUID gets a handle to a particular device by UUIC
func DeviceHandleByUUID(uuid string) (Device, error) {
	var dev C.hlml_device_t

	cstr := C.CString(uuid)
	defer C.free(unsafe.Pointer(cstr))

	rc := C.hlml_device_get_handle_by_UUID(cstr, &dev)
	return Device{dev}, errorString(rc)
}

// DeviceHandleBySerial gets a handle to a particular device by serial number
func DeviceHandleBySerial(serial string) (*Device, error) {
	numDevices, _ := DeviceCount()

	for i := uint(0); i < numDevices; i++ {
		handle, _ := DeviceHandleByIndex(i)

		currentSerial, _ := handle.SerialNumber()

		if currentSerial == serial {
			return &handle, nil
		}
	}

	return nil, errors.New("could not find device with serial number")
}

// MinorNumber returns Minor number.
func (d Device) MinorNumber() (uint, error) {
	var minor C.uint

	rc := C.hlml_device_get_minor_number(d.dev, &minor)
	return uint(minor), errorString(rc)
}

// Name returns Device Name
func (d Device) Name() (string, error) {
	var name [szUUID]C.char

	rc := C.hlml_device_get_name(d.dev, &name[0], szUUID)
	return C.GoString(&name[0]), errorString(rc)
}

// UUID returns the unique id for a given device
func (d Device) UUID() (string, error) {
	var uuid [szUUID]C.char

	rc := C.hlml_device_get_uuid(d.dev, &uuid[0], szUUID)
	return C.GoString(&uuid[0]), errorString(rc)
}

// PCIDomain returns the PCI domain for a given device
func (d Device) PCIDomain() (uint, error) {
	var pci C.hlml_pci_info_t

	rc := C.hlml_device_get_pci_info(d.dev, &pci)
	return uint(pci.domain), errorString(rc)
}

// PCIBus returns the PCI bus info for a given device
func (d Device) PCIBus() (uint, error) {
	var pci C.hlml_pci_info_t

	rc := C.hlml_device_get_pci_info(d.dev, &pci)
	return uint(pci.bus), errorString(rc)
}

// PCIBusID returns the PCI bus id for a given device
func (d Device) PCIBusID() (string, error) {
	var pci C.hlml_pci_info_t

	rc := C.hlml_device_get_pci_info(d.dev, &pci)
	return C.GoString(&pci.bus_id[0]), errorString(rc)
}

// PCIID returns the PCI id for a given device
func (d Device) PCIID() (uint, error) {
	var pci C.hlml_pci_info_t

	rc := C.hlml_device_get_pci_info(d.dev, &pci)
	return uint(pci.pci_device_id), errorString(rc)
}

// PCILinkSpeed returns the current PCI link speed for a given device
func (d Device) PCILinkSpeed() (uint, error) {
	var pci C.hlml_pci_info_t

	rc := C.hlml_device_get_pci_info(d.dev, &pci)
	speed := C.GoString(&pci.caps.link_speed[0])
	speed = strings.ReplaceAll(speed, "0x", "")
	res, _ := strconv.Atoi(speed)
	return uint(res), errorString(rc)
}

// PCILinkWidth returns the current PCI link width for a given device
func (d Device) PCILinkWidth() (uint, error) {
	var pci C.hlml_pci_info_t

	rc := C.hlml_device_get_pci_info(d.dev, &pci)
	width := C.GoString(&pci.caps.link_width[0])
	res, _ := strconv.Atoi(width)
	return uint(res), errorString(rc)
}

// MemoryInfo returns the current memory usage in bytes for total, used, free
func (d Device) MemoryInfo() (uint64, uint64, uint64, error) {
	var mem C.hlml_memory_t
	rc := C.hlml_device_get_memory_info(d.dev, &mem)
	return uint64(mem.total), uint64(mem.used), uint64(mem.total - mem.used), errorString(rc)
}

// UtilizationInfo returns the utilization aip rate for a given device
func (d Device) UtilizationInfo() (uint, error) {
	var util C.hlml_utilization_t

	rc := C.hlml_device_get_utilization_rates(d.dev, &util)
	return uint(util.aip), errorString(rc)
}

// SOCClockInfo returns the SoC clock frequency for a given device
func (d Device) SOCClockInfo() (uint, error) {
	var freq C.uint

	rc := C.hlml_device_get_clock_info(d.dev, C.HLML_CLOCK_SOC, &freq)
	return uint(freq), errorString(rc)
}

// SOCClockMax returns the maximum SoC clock frequency for a given device
func (d Device) SOCClockMax() (uint, error) {
	var freq C.uint
	rc := C.hlml_device_get_max_clock_info(d.dev, C.HLML_CLOCK_SOC, &freq)
	return uint(freq), errorString(rc)
}

// ICClockMax returns the maximum IC clock frequency for a given device
func (d Device) ICClockMax() (uint, error) {
	var freq C.uint
	rc := C.hlml_device_get_max_clock_info(d.dev, C.HLML_CLOCK_IC, &freq)
	return uint(freq), errorString(rc)
}

// MMEClockMax returns the maximum MME clock frequency for a given device
func (d Device) MMEClockMax() (uint, error) {
	var freq C.uint
	rc := C.hlml_device_get_max_clock_info(d.dev, C.HLML_CLOCK_MME, &freq)
	return uint(freq), errorString(rc)
}

// TPCClockMax returns the maximum TPC clock frequency for a given device
func (d Device) TPCClockMax() (uint, error) {
	var freq C.uint
	rc := C.hlml_device_get_max_clock_info(d.dev, C.HLML_CLOCK_TPC, &freq)
	return uint(freq), errorString(rc)
}

// PowerUsage returns the power usage in milliwatts for a given device
func (d Device) PowerUsage() (uint, error) {
	var power C.uint
	rc := C.hlml_device_get_power_usage(d.dev, &power)
	return uint(power), errorString(rc)
}

// TemperatureOnBoard returns the temperature in celsius for a device board
func (d Device) TemperatureOnBoard() (uint, error) {
	var onBoard C.uint
	rc := C.hlml_device_get_temperature(d.dev, C.HLML_TEMPERATURE_ON_BOARD, &onBoard)
	return uint(onBoard), errorString(rc)
}

// TemperatureOnChip returns the temperature in celsius for a the device chip
func (d Device) TemperatureOnChip() (uint, error) {
	var onChip C.uint
	rc := C.hlml_device_get_temperature(d.dev, C.HLML_TEMPERATURE_ON_AIP, &onChip)
	return uint(onChip), errorString(rc)
}

// TemperatureThresholdShutdown Retrieves the known temperature threshold for the AIP with the specified threshold type in degrees
func (d Device) TemperatureThresholdShutdown() (uint, error) {
	var temp C.uint
	rc := C.hlml_device_get_temperature_threshold(d.dev, C.HLML_TEMPERATURE_THRESHOLD_SHUTDOWN, &temp)
	return uint(temp), errorString(rc)
}

// TemperatureThresholdSlowdown Retrieves the known temperature threshold for the AIP with the specified threshold type in degrees
func (d Device) TemperatureThresholdSlowdown() (uint, error) {
	var temp C.uint
	rc := C.hlml_device_get_temperature_threshold(d.dev, C.HLML_TEMPERATURE_THRESHOLD_SLOWDOWN, &temp)
	return uint(temp), errorString(rc)
}

// TemperatureThresholdMemory Retrieves the known temperature threshold for the AIP with the specified threshold type in degrees
func (d Device) TemperatureThresholdMemory() (uint, error) {
	var temp C.uint
	rc := C.hlml_device_get_temperature_threshold(d.dev, C.HLML_TEMPERATURE_THRESHOLD_MEM_MAX, &temp)
	return uint(temp), errorString(rc)
}

// TemperatureThresholdGPU Retrieves the known temperature threshold for the AIP with the specified threshold type in degrees
func (d Device) TemperatureThresholdGPU() (uint, error) {
	var temp C.uint
	rc := C.hlml_device_get_temperature_threshold(d.dev, C.HLML_TEMPERATURE_THRESHOLD_GPU_MAX, &temp)
	return uint(temp), errorString(rc)
}

// PowerManagementDefaultLimit Retrieves default power management limit on this device, in milliwatts.
// Default power management limit is a power management limit that the device boots with.
func (d Device) PowerManagementDefaultLimit() (uint, error) {
	var limit C.uint
	rc := C.hlml_device_get_power_management_default_limit(d.dev, &limit)
	return uint(limit), errorString(rc)
}

// ECCMode retrieves the current and pending ECC modes for the device
//
//	1 - ECCMode enabled
//	0 - ECCMode disabled
func (d Device) ECCMode() (uint, uint, error) {
	var current, pending C.hlml_enable_state_t
	rc := C.hlml_device_get_ecc_mode(d.dev, &current, &pending)
	return uint(current), uint(pending), errorString(rc)
}

// HLRevision returns the revision of the HL library
func (d Device) HLRevision() (int, error) {
	var rev C.int
	rc := C.hlml_device_get_hl_revision(d.dev, &rev)
	return int(rev), errorString(rc)
}

// PCBVersion returns the PCB version
func (d Device) PCBVersion() (string, error) {
	var pcb C.hlml_pcb_info_t

	rc := C.hlml_device_get_pcb_info(d.dev, &pcb)
	return C.GoString(&pcb.pcb_ver[0]), errorString(rc)
}

// PCBAssemblyVersion returns the PCB Assembly info
func (d Device) PCBAssemblyVersion() (string, error) {
	var pcb C.hlml_pcb_info_t

	rc := C.hlml_device_get_pcb_info(d.dev, &pcb)
	return C.GoString(&pcb.pcb_assembly_ver[0]), errorString(rc)
}

// SerialNumber returns the device serial number
func (d Device) SerialNumber() (string, error) {
	var serial [szUUID]C.char

	rc := C.hlml_device_get_serial(d.dev, &serial[0], szUUID)
	return C.GoString(&serial[0]), errorString(rc)
}

// ModuleID returns the device moduleID
func (d Device) ModuleID() (uint, error) {
	var moduleID C.uint
	rc := C.hlml_device_get_module_id(d.dev, &moduleID)
	return uint(moduleID), errorString(rc)
}

// BoardID returns an ID for the PCB board
func (d Device) BoardID() (uint, error) {
	var id C.uint

	rc := C.hlml_device_get_board_id(d.dev, &id)
	return uint(id), errorString(rc)
}

// PCIeTX returns PCIe transmit throughput
func (d Device) PCIeTX() (uint, error) {
	var val C.uint

	rc := C.hlml_device_get_pcie_throughput(d.dev, C.HLML_PCIE_UTIL_TX_BYTES, &val)
	return uint(val), errorString(rc)
}

// PCIeRX returns PCIe receive throughput
func (d Device) PCIeRX() (uint, error) {
	var val C.uint

	rc := C.hlml_device_get_pcie_throughput(d.dev, C.HLML_PCIE_UTIL_RX_BYTES, &val)
	return uint(val), errorString(rc)
}

// PCIReplayCounter returns PCIe replay count
func (d Device) PCIReplayCounter() (uint, error) {
	var val C.uint

	rc := C.hlml_device_get_pcie_replay_counter(d.dev, &val)
	return uint(val), errorString(rc)
}

// PCIeLinkGeneration returns PCIe replay count
// MUST run with SUDO/priviledged
func (d Device) PCIeLinkGeneration() (uint, error) {
	var gen C.uint

	rc := C.hlml_device_get_curr_pcie_link_generation(d.dev, &gen)
	return uint(gen), errorString(rc)
}

// PCIeLinkWidth returns PCIe link width
func (d Device) PCIeLinkWidth() (uint, error) {
	var width C.uint

	rc := C.hlml_device_get_curr_pcie_link_width(d.dev, &width)
	return uint(width), errorString(rc)
}

// ClockThrottleReasons returns current clock throttle reasons
func (d Device) ClockThrottleReasons() (uint64, error) {
	var reasons C.ulonglong

	rc := C.hlml_device_get_current_clocks_throttle_reasons(d.dev, &reasons)
	return uint64(reasons), errorString(rc)
}

// EnergyConsumptionCounter returns energy consumption
func (d Device) EnergyConsumptionCounter() (uint64, error) {
	var energy C.ulonglong

	rc := C.hlml_device_get_total_energy_consumption(d.dev, &energy)
	return uint64(energy), errorString(rc)
}

// MacAddressInfo retrieves the masks for supported ports and external ports.
func (d Device) MacAddressInfo() (map[int]string, error) {
	var mask [C.PORTS_ARR_SIZE]C.uint64_t
	var extMask [C.PORTS_ARR_SIZE]C.uint64_t

	rc := C.hlml_get_mac_addr_info(d.dev, &mask[0], &extMask[0])

	ports := make(map[int]string)
	maskBinary := strconv.FormatInt(int64(mask[0]), 2)
	extMaskBinary := strconv.FormatInt(int64(extMask[0]), 2)
	for i := len(maskBinary) - 1; i >= 0; i-- {
		if maskBinary[i] == 49 && extMaskBinary[i] == 49 {
			ports[len(maskBinary)-i-1] = "external"
		} else if maskBinary[i] == 49 && extMaskBinary[i] == 48 {
			ports[len(extMaskBinary)-i-1] = "internal"
		}
	}

	return ports, errorString(rc)
}

// NicLinkStatus gets a port and checks its status.
// return 1 (up) or 0 (down)
func (d Device) NicLinkStatus(port uint) (uint, error) {
	var up C.bool
	rc := C.hlml_nic_get_link(d.dev, C.uint(port), &up)
	if up {
		return uint(1), errorString(rc)
	}
	return uint(0), errorString(rc)
}

// ReplacedRowDoubleBitECC returns the number of rows with double-bit ecc errors
func (d Device) ReplacedRowDoubleBitECC() (uint, error) {
	var rowsCount C.uint = 0
	rc := C.hlml_device_get_replaced_rows(d.dev, C.HLML_ROW_REPLACEMENT_CAUSE_DOUBLE_BIT_ECC_ERROR, &rowsCount, nil)
	return uint(rowsCount), errorString(rc)
}

// ReplacedRowSingleBitECC returns the number of rows with single-bit ecc errors
func (d Device) ReplacedRowSingleBitECC() (uint, error) {
	var rowsCount C.uint
	rc := C.hlml_device_get_replaced_rows(d.dev, C.HLML_ROW_REPLACEMENT_CAUSE_MULTIPLE_SINGLE_BIT_ECC_ERRORS, &rowsCount, nil)
	return uint(rowsCount), errorString(rc)
}

// IsReplacedRowsPendingStatus return 0 (false) or 1 (true) if there are any
// rows need of replacement in a power cycle
func (d Device) IsReplacedRowsPendingStatus() (int, error) {
	var isPending C.hlml_enable_state_t
	rc := C.hlml_device_get_replaced_rows_pending_status(d.dev, &isPending)
	return int(isPending), errorString(rc)
}

// NumaNode returns the Numa affinity of the device or nil is no affinity.
func (d Device) NumaNode() (*uint, error) {
	busID, err := d.PCIBusID()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(fmt.Sprintf("/sys/bus/pci/devices/%s/numa_node", strings.ToLower(busID)))
	if err != nil {
		// report nil if NUMA support isn't enabled
		return nil, nil
	}
	node, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 8)
	if err != nil {
		return nil, fmt.Errorf("%v: %v", errors.New("failed to retrieve CPU affinity"), err)
	}
	if node < 0 {
		return nil, nil
	}

	numaNode := uint(node)
	return &numaNode, nil
}

// FWVersion returns the firmware version for a given device
func FWVersion(idx uint) (kernel string, uboot string, err error) {
	b, err := os.ReadFile(fmt.Sprintf("%s/accel%d/device/armcp_kernel_ver", HLDriverPath, idx))
	if err != nil {
		return "", "", fmt.Errorf("file reading error %s", err)
	}
	kernel = string(b)

	b, err = os.ReadFile(fmt.Sprintf("%s/accel%d/device/uboot_ver", HLDriverPath, idx))
	if err != nil {
		return "", "", fmt.Errorf("file reading error %s", err)
	}
	uboot = string(b)

	return kernel, uboot, nil
}

// SystemDriverVersion returns the driver version on the system
func SystemDriverVersion() (string, error) {
	driver, err := os.ReadFile(HLModulePath + "/version")
	if err != nil {
		return "", fmt.Errorf("file reading error %s", err)
	}
	return string(driver), nil
}

func NewEventSet() EventSet {
	var set C.hlml_event_set_t
	C.hlml_event_set_create(&set)

	return EventSet{set}
}

func RegisterEventForDevice(es EventSet, event int, uuid string) error {
	deviceHandle, err := DeviceHandleBySerial(uuid)
	if err != nil {
		return fmt.Errorf("hlml: device not found")
	}

	r := C.hlml_device_register_events(deviceHandle.dev, C.ulonglong(event), es.set)
	if r != C.HLML_SUCCESS {
		return errorString(r)
	}

	return nil
}

func DeleteEventSet(es EventSet) {
	C.hlml_event_set_free(es.set)
}

func WaitForEvent(es EventSet, timeout uint) (Event, error) {
	var data C.hlml_event_data_t

	r := C.hlml_event_set_wait(es.set, &data, C.uint(timeout))
	serial, _ := Device{data.device}.SerialNumber()

	return Event{
			Serial: serial,
			Etype:  uint64(data.event_type),
		},
		errorString(r)
}

func GetDeviceTypeName() (string, error) {
	var deviceType string

	err := filepath.Walk(pciBasePath, func(path string, info os.FileInfo, err error) error {
		log.Println(pciBasePath, info.Name())
		if err != nil {
			return fmt.Errorf("error accessing file path %q", path)
		}
		if info.IsDir() {
			log.Println("Not a device, continuing")
			return nil
		}
		// Retrieve vendor for the device
		vendorID, err := readIDFromFile(pciBasePath, info.Name(), "vendor")
		if err != nil {
			return fmt.Errorf("get vendor: %w", err)
		}

		// Habana vendor id is "1da3".
		if vendorID != "1da3" {
			return nil
		}

		deviceID, err := readIDFromFile(pciBasePath, info.Name(), "device")
		if err != nil {
			return fmt.Errorf("get device info: %w", err)
		}

		deviceType, err = getDeviceName(deviceID)
		if err != nil {
			return fmt.Errorf("get device name: %w", err)
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return deviceType, nil
}

func getDeviceName(deviceID string) (string, error) {
	goya := []string{"0001"}
	// Gaudi family includes Gaudi 1 and Guadi 2
	gaudi := []string{"1000", "1001", "1010", "1011", "1020", "1030", "1060", "1061", "1062"}
	greco := []string{"0020", "0030"}

	switch {
	case checkFamily(goya, deviceID):
		return "goya", nil
	case checkFamily(gaudi, deviceID):
		return "gaudi", nil
	case checkFamily(greco, deviceID):
		return "greco", nil
	default:
		return "", errors.New("no habana devices on the system")
	}
}

func checkFamily(family []string, id string) bool {
	for _, m := range family {
		if strings.HasSuffix(id, m) {
			return true
		}
	}
	return false
}

func readIDFromFile(basePath string, deviceAddress string, property string) (string, error) {
	data, err := os.ReadFile(filepath.Join(basePath, deviceAddress, property))
	if err != nil {
		return "", fmt.Errorf("could not read %s for device %s: %w", property, deviceAddress, err)
	}
	id := strings.Trim(string(data[2:]), "\n")
	return id, nil
}
