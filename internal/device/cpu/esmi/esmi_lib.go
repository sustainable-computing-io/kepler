package esmi

import (
	"sync"
)

/*
#cgo CFLAGS: -I/opt/rocm/include -I/opt/e-sms/e_smi/include
#cgo LDFLAGS: -L/opt/rocm/lib -lamd_smi

#include <stdint.h>
#include <stdlib.h>

// NOTE: Adjust header if your system uses amd_smi/esmi_ib
#include <amd_smi/amdsmi.h>
#include <e_smi/e_smi.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

var initOnce sync.Once
var initErr error
//var (
//	socketHandles []C.amdsmi_socket_handle
//	socketCount   uint32
//)

func Init() error {
	initOnce.Do(func() {
		ret := C.amdsmi_init(0)
		if ret != 0 {
			initErr = fmt.Errorf("amdsmi_init failed: %d", int(ret))
			return
		}

		e_ret := C.esmi_init()
		if e_ret != 0 {
			initErr = fmt.Errorf("esmi_init failed: %d", int(e_ret))
			return
		}

//		var count C.uint32_t
//		ret = C.amdsmi_get_socket_handles(&count, nil)
//		if ret != 0 {
//			initErr = fmt.Errorf("amdsmi_get_socket_handles(count) failed: %d", int(ret))
//			return
//		}
//
//		if count == 0 {
//			initErr = fmt.Errorf("no sockets found")
//			return
//		}
//
//		// Allocate slice
//		socketHandles = make([]C.amdsmi_socket_handle, count)
//
//		// Second call → get handles
//		ret = C.amdsmi_get_socket_handles(&count, &socketHandles[0])
//		if ret != 0 {
//			initErr = fmt.Errorf("amdsmi_get_socket_handles(handles) failed: %d", int(ret))
//			return
//		}
//
//		socketCount = uint32(count)

	})
	return initErr
}

func Close() {
	C.amdsmi_shut_down()
}

func extractDimmPower(dp C.struct_dimm_power) float64 {
	val := *(*C.uint32_t)(unsafe.Pointer(&dp))
	// dp.power is fixed-point → upper bits
	return float64((val >> 17) & 0x7FFF)
}

// ----------------------
// Socket-level power
// ----------------------
func GetSocketPower(socket int) (float64, error) {
	var power C.uint32_t

	//ret := C.amdsmi_get_cpu_socket_power(socketHandles[socket], &power)
	ret := C.esmi_socket_power_get(C.uint32_t(socket), &power)
	if ret != 0 {
		return 0, fmt.Errorf("esmi_get_socket_power failed: %d", int(ret))
	}

	return float64(power), nil
}

func GetSocketEnergy(socket int) (float64, error) {
	var energy C.uint64_t

	ret := C.esmi_socket_energy_get(C.uint32_t(socket), &energy)
	if ret != 0 {
		return 0, fmt.Errorf("esmi_socket_energy_get failed: %d", int(ret))
	}

	return float64(energy), nil
}
func GetSocketMaxEnergy(socket int) (float64, error) {
	// currently not supported
	return 0, nil
}

// ----------------------
// DRAM power
// ----------------------
func GetDramPower(socket int) (float64, error) {
	return 0, nil
	//var dp C.struct_dimm_power
	//var opts C.union_dimm_power_inarg

	//var reg uint32
	//reg |= uint32(0) // dimm_addr
	//reg |= uint32(C.TOTAL_DIMM_POWER) << 30

	//*(*C.uint32_t)(unsafe.Pointer(&opts)) = C.uint32_t(reg)

	//ret := C.esmi_dimm_power_consumption_data_get(
	//    C.uint8_t(socket),
	//    opts,
	//    &dp,
	//)
	//if ret != 0 {
	//    return 0, fmt.Errorf("esmi_get_dram_power failed: %d", int(ret))
	//}

	//return extractDimmPower(dp), nil
}

func GetDramMaxPower(socket int) (float64, error) {
	return 0, nil
	//var dp C.struct_dimm_power
	//var opts C.union_dimm_power_inarg

	//var reg uint32
	//reg |= uint32(0) // dimm_addr
	//reg |= uint32(C.MAX_DIMM_POWER) << 30

	//*(*C.uint32_t)(unsafe.Pointer(&opts)) = C.uint32_t(reg)

	//ret := C.esmi_dimm_power_consumption_data_get(
	//    C.uint8_t(socket),
	//    opts,
	//    &dp,
	//)
	//if ret != 0 {
	//    return 0, fmt.Errorf("esmi_get_dram_power failed: %d", int(ret))
	//}

	//return extractDimmPower(dp), nil
}

// ----------------------
// Core-level Energy
// ----------------------
func GetCoreEnergy(core int) (float64, error) {
	var energy C.uint64_t

	ret := C.esmi_core_energy_get(C.uint32_t(core), &energy)
	if ret != 0 {
		return 0, fmt.Errorf("esmi_core_energy_get failed: %d", int(ret))
	}

	return float64(energy), nil
}

func GetCoreMaxEnergy(core int) (float64, error) {
	//currently not supported
	return 0, nil
}

func GetCorePower(core int) (float64, error) {
	//currently not supported
	return 0, nil
}

// ----------------------
// Socket count
// ----------------------
func GetSocketCount() (int, error) {
	var count C.uint32_t

	ret := C.esmi_number_of_sockets_get(&count)
        if ret != 0 {
                return 0, fmt.Errorf("esmi_number_of_sockets_get: %d", int(ret))
        }

        return int(count), nil
}

// ----------------------
// Core count
// ----------------------
func GetCoreCount() (int, error) {
	var cpuCount C.uint32_t
	var smtThreads C.uint32_t

	ret := C.esmi_number_of_cpus_get(&cpuCount)
	if ret != 0 {
		return 0, fmt.Errorf("esmi_number_of_cpus_get failed: %d", int(ret))
	}

	ret = C.esmi_threads_per_core_get(&smtThreads)
	if ret != 0 {
		return 0, fmt.Errorf("eesmi_threads_per_core_get failed: %d", int(ret))
	}

	return int(cpuCount / smtThreads), nil
}
