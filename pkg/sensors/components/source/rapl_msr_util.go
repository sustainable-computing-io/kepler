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
	"syscall"

	"github.com/jaypipes/ghw"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"k8s.io/klog/v2"
)

const (
	msrPath = "/dev/cpu/%d/msr"

	msrRaplPowerUnit    = 0x00000606
	msrPkgEnergyStatus  = 0x00000611
	msrDramEnergyStatus = 0x00000619
	msrPP0EnergyStatus  = 0x00000639
	msrPP1EnergyStatus  = 0x00000641
)

var (
	cpu       *ghw.CPUInfo
	fds       []int
	byteOrder binary.ByteOrder

	numPackages, numCores int

	initCompleted bool

	// powerUnits and timeUnits not used yet in Kepler, annotate here for future use.
	// powerUnits, timeUnits float64
	// energyStatusUnits should be package specific, but normally the same among packages
	energyStatusUnits []float64
)

func init() {
	byteOrder = utils.DetermineHostByteOrder()
	var err error
	cpu, err = ghw.CPU()
	if cpu == nil || err != nil {
		fmt.Printf("Error getting CPU info: %v", err)
		// For ghw lib not applicable platform or other corner cases, simply keep the numPackages and numCores as 0.
	} else {
		numPackages = len(cpu.Processors)
		numCores = int(cpu.TotalThreads)
	}
	initCompleted = false
}

func OpenAllMSR() error {
	if numCores == 0 {
		return fmt.Errorf("failed to initialze cpu info")
	}
	fds = make([]int, numCores)
	for c := 0; c < numCores; c++ {
		path := fmt.Sprintf(msrPath, c)
		fd, err := syscall.Open(path, syscall.O_RDONLY, 777)
		if err != nil {
			return fmt.Errorf("failed to open path %s: %v", path, err)
		}
		fds[c] = fd
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
	if packageID >= numPackages {
		return 0, fmt.Errorf("package Id %d greater than max package id %d", packageID, numPackages-1)
	}
	if cpu.Processors[packageID].NumCores == 0 || cpu.Processors[packageID].NumThreads == 0 {
		return 0, fmt.Errorf("no cpu core/hardware thread in package %d", packageID)
	}
	// Currently the cores in the same CPU Package(Socket) have the same RAPL MSR value
	core := cpu.Processors[packageID].Cores[0].LogicalProcessors[0]
	buf := make([]byte, 8)
	bytes, err := syscall.Pread(fds[core], buf, msr)

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
	if initCompleted {
		return nil
	}

	if err := OpenAllMSR(); err != nil {
		klog.V(1).Info(err)
		return err
	}
	energyStatusUnits = make([]float64, numPackages)
	for i := 0; i < numPackages; i++ {
		result, err := ReadMSR(i, msrRaplPowerUnit)
		if err != nil {
			klog.V(1).Info(err)
			return fmt.Errorf("failed to read power unit: %v", err)
		}
		// See definition in "Intel® 64 and IA-32 Architectures Software Developer’s Manual" Section 15.10.1, Figure 15-35
		// powerUnits and timeUnits not used yet in Kepler, annotate here for future use.
		// powerUnits = math.Pow(0.5, float64((result & 0xf)))
		// timeUnits = math.Pow(0.5, float64(((result >> 16) & 0xf)))
		energyStatusUnits[i] = math.Pow(0.5, float64(((result >> 8) & 0x1f)))
	}
	initCompleted = true
	return nil
}

func ReadPkgPower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrPkgEnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read pkg energy: %v", err)
	}
	return uint64(energyStatusUnits[packageID] * float64(result) * 1000 /*mJ*/), nil
}

func ReadCorePower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrPP0EnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read pp0 energy: %v", err)
	}
	return uint64(energyStatusUnits[packageID] * float64(result) * 1000 /*mJ*/), nil
}

func ReadUncorePower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrPP1EnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read pp1 energy: %v", err)
	}
	return uint64(energyStatusUnits[packageID] * float64(result) * 1000 /*mJ*/), nil
}

func ReadDramPower(packageID int) (uint64, error) {
	result, err := ReadMSR(packageID, msrDramEnergyStatus)
	if err != nil {
		return 0, fmt.Errorf("failed to read dram energy: %v", err)
	}
	return uint64(energyStatusUnits[packageID] * float64(result) * 1000 /*mJ*/), nil
}

func ReadAllPower(f func(n int) (uint64, error)) (uint64, error) {
	energy := uint64(0)
	for i := 0; i < numPackages; i++ {
		result, err := f(i)
		if err != nil {
			return 0, err
		}
		energy += result
	}
	return energy, nil
}

func GetRAPLEnergyByMSR(coreFunc, dramFunc, uncoreFunc, pkgFunc func(n int) (uint64, error)) map[int]NodeComponentsEnergy {
	packageEnergies := make(map[int]NodeComponentsEnergy)
	for i := 0; i < numPackages; i++ {
		coreEnergy, _ := coreFunc(i)
		dramEnergy, _ := dramFunc(i)
		uncoreEnergy, _ := uncoreFunc(i)
		pkgEnergy, _ := pkgFunc(i)
		packageEnergies[i] = NodeComponentsEnergy{
			Core:   coreEnergy,
			DRAM:   dramEnergy,
			Uncore: uncoreEnergy,
			Pkg:    pkgEnergy,
		}
	}
	return packageEnergies
}
