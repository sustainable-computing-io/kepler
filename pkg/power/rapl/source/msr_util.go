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

/*
 Inspired by the following source(s)
 https://web.eece.maine.edu/~vweaver/projects/rapl/rapl-read.c
*/

package source

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	msrPath      = "/dev/cpu/%d/msr"
	topologyPath = "/sys/devices/system/cpu/cpu%d/topology/physical_package_id"

	msrRaplPowerUnit    = 0x00000606
	msrPkgEnergyStatus  = 0x00000611
	msrDramEnergyStatus = 0x00000619
	msrPP0EnergyStatus  = 0x00000639
	msrPP1EnergyStatus  = 0x00000641
)

var (
	fds        []int
	byteOrder  binary.ByteOrder
	packageMap []int
	maxPackage = -1

	powerUnits, timeUnits           float64
	cpuEnergyUnits, dramEnergyUnits []float64
)

func init() {
	var i int32 = 0x01020304
	u := unsafe.Pointer(&i)
	pb := (*byte)(u)
	b := *pb
	if b == 0x04 {
		byteOrder = binary.LittleEndian
	} else {
		byteOrder = binary.BigEndian
	}
}

func mapPackageAndCore() error {
	cores := runtime.NumCPU()
	packageMap = make([]int, cores)

	for i := 0; i < cores; {
		packageMap[i] = -1
		i++
	}

	for i := 0; i < cores; {
		path := fmt.Sprintf(topologyPath, i)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read topology %s: %v", path, err)
		}
		id, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return err
		}
		if packageMap[id] == -1 {
			packageMap[id] = i
		}
		if maxPackage < id {
			maxPackage = id
		}
		i++
	}
	return nil
}

func OpenAllMSR() error {
	fds = make([]int, maxPackage+1)
	for i := 0; i <= maxPackage; {
		core := packageMap[i]
		path := fmt.Sprintf(msrPath, core)
		fd, err := syscall.Open(path, syscall.O_RDONLY, 777)
		if err != nil {
			return fmt.Errorf("failed to open path %s: %v", path, err)
		}
		fds[i] = fd
		i++
	}
	return nil
}

func CloseAllMSR() {
	for _, v := range fds {
		if v != 0 {
			syscall.Close(v)
		}
	}
}

func ReadMSR(packageID int, msr int64) (uint64, error) {
	if packageID > maxPackage {
		return 0, fmt.Errorf("package Id %d greater than max package id %d", packageID, maxPackage)
	}
	buf := make([]byte, 8)
	core := packageMap[packageID]
	if core == -1 || fds[packageID] == 0 {
		return 0, fmt.Errorf("no cpu core or msr found in package %d", packageID)
	}
	bytes, err := syscall.Pread(fds[packageID], buf, msr)

	if err != nil {
		return 0, err
	}

	if bytes != 8 {
		return 0, fmt.Errorf("wrong bytes: %d", bytes)
	}

	msrVal := byteOrder.Uint64(buf)

	return msrVal, nil
}

func InitUnits() error {
	if err := mapPackageAndCore(); err != nil {
		return err
	}
	if err := OpenAllMSR(); err != nil {
		return err
	}
	cpuEnergyUnits = make([]float64, maxPackage+1)
	dramEnergyUnits = make([]float64, maxPackage+1)
	for i := 0; i <= maxPackage; {
		result, err := ReadMSR(i, msrRaplPowerUnit)
		if err != nil {
			return fmt.Errorf("failed to read power unit: %v", err)
		}
		powerUnits = math.Pow(0.5, float64((result & 0xf)))
		timeUnits = math.Pow(0.5, float64(((result >> 16) & 0xf)))
		cpuEnergyUnits[i] = 1 / math.Pow(2, float64((result&0x1f00)>>8))
		dramEnergyUnits[i] = math.Pow(0.5, float64(((result >> 8) & 0x1f)))
		i++
	}
	return nil
}

func ReadPkgPower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrPkgEnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read pkg energy: %v", err)
	}
	return uint64(cpuEnergyUnits[packageID] * float64(result)), nil
}

func ReadCorePower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrPP0EnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read pp0 energy: %v", err)
	}
	return uint64(cpuEnergyUnits[packageID] * float64(result) * 1000 /*mJ*/), nil
}

func ReadUncorePower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrPP1EnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read pp1 energy: %v", err)
	}
	return uint64(cpuEnergyUnits[packageID] * float64(result) * 1000 /*mJ*/), nil
}

func ReadDramPower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrDramEnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read dram energy: %v", err)
	}
	return uint64(dramEnergyUnits[packageID] * float64(result) * 1000 /*mJ*/), nil
}

func ReadAllPower(f func(n int) (uint64, error)) (uint64, error) {
	energy := uint64(0)
	for i := 0; i <= maxPackage; {
		result, err := f(i)
		if err != nil {
			return 0, err
		}
		energy += result
		i++
	}
	return energy, nil
}

func GetRAPLEnergyByMSR(coreFunc, dramFunc, uncoreFunc, pkgFunc func(n int) (uint64, error)) map[int]RAPLEnergy {
	packageEnergies := make(map[int]RAPLEnergy)
	for i := 0; i <= maxPackage; {
		coreEnergy, _ := coreFunc(i)
		dramEnergy, _ := dramFunc(i)
		uncoreEnergy, _ := uncoreFunc(i)
		pkgEnergy, _ := pkgFunc(i)
		packageEnergies[i] = RAPLEnergy{
			Core:   coreEnergy,
			DRAM:   dramEnergy,
			Uncore: uncoreEnergy,
			Pkg:    pkgEnergy,
		}
		i++
	}
	return packageEnergies
}
