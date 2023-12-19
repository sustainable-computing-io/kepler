/*
Copyright 2023.

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

package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
)

const (
	uJTomJ        = 1000
	resultDirPath = "/output"
)

var (
	genEnv         = flag.Bool("gen-env", false, "whether generate platform-validaion.env or not")
	genPower       = flag.Bool("gen-components-power", true, "whether generate components mean power or not")
	sampleCount    = flag.Int("sample-count", 20, "power sampling number")
	sampleDuration = flag.Int("sample-duration", 15, "sampling duration in seconds")
	pkgNum         int
	pkgMax         uint64
	coreMax        uint64
	uncoreMax      uint64
	dramMax        uint64
	samplePowers   []NodeComponentsPower
	meanPower      NodeComponentsPower
)

type NodeComponentsPower struct {
	pkgPower    float64
	corePower   float64
	uncorePower float64
	dramPower   float64
}

func calculateNodeComponentsPower(index int, pre, cur map[int]source.NodeComponentsEnergy) {
	var (
		prePkgTotal, curPkgTotal       uint64
		preCoreTotal, curCoreTotal     uint64
		preUncoreTotal, curUncoreTotal uint64
		preDramTotal, curDramTotal     uint64
	)
	for i := 0; i < pkgNum; i++ {
		prePkgTotal += pre[i].Pkg
		if pre[i].Pkg > cur[i].Pkg {
			curPkgTotal += cur[i].Pkg + pkgMax
		} else {
			curPkgTotal += cur[i].Pkg
		}
		preCoreTotal += pre[i].Core
		if pre[i].Core > cur[i].Core {
			curCoreTotal += cur[i].Core + coreMax
		} else {
			curCoreTotal += cur[i].Core
		}
		preUncoreTotal += pre[i].Uncore
		if pre[i].Uncore > cur[i].Uncore {
			curUncoreTotal += cur[i].Uncore + uncoreMax
		} else {
			curUncoreTotal += cur[i].Uncore
		}
		preDramTotal += pre[i].DRAM
		if pre[i].DRAM > cur[i].DRAM {
			curDramTotal += cur[i].DRAM + dramMax
		} else {
			curDramTotal += cur[i].DRAM
		}
	}
	samplePowers[index].pkgPower = float64(curPkgTotal-prePkgTotal) / float64(uJTomJ) / float64(*sampleDuration)
	samplePowers[index].corePower = float64(curCoreTotal-preCoreTotal) / float64(uJTomJ) / float64(*sampleDuration)
	samplePowers[index].uncorePower = float64(curUncoreTotal-preUncoreTotal) / float64(uJTomJ) / float64(*sampleDuration)
	samplePowers[index].dramPower = float64(curDramTotal-preDramTotal) / float64(uJTomJ) / float64(*sampleDuration)
}

func updateNodePower(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < *sampleCount; i++ {
			pre := components.GetAbsEnergyFromNodeComponents()
			time.Sleep(time.Duration(*sampleDuration) * time.Second)
			cur := components.GetAbsEnergyFromNodeComponents()
			fmt.Printf("Sample %d:\n", i+1)
			fmt.Printf("pre: %v\ncur: %v\n", pre, cur)
			calculateNodeComponentsPower(i, pre, cur)
			meanPower.pkgPower += samplePowers[i].pkgPower
			meanPower.corePower += samplePowers[i].corePower
			meanPower.uncorePower += samplePowers[i].uncorePower
			meanPower.dramPower += samplePowers[i].dramPower
		}
		meanPower.pkgPower /= float64(*sampleCount)
		meanPower.corePower /= float64(*sampleCount)
		meanPower.uncorePower /= float64(*sampleCount)
		meanPower.dramPower /= float64(*sampleCount)
		fmt.Printf("Dump mean power:\n")
		fmt.Printf("pkg:%f\ncore:%f\nuncore:%f\ndram:%f\n", meanPower.pkgPower, meanPower.corePower, meanPower.uncorePower, meanPower.dramPower)
	}()
}

func getX86Architecture() (string, error) {
	// use cpuid to get CPUArchitecture
	grep := exec.Command("grep", "uarch")
	output := exec.Command("cpuid", "-1")
	pipe, err := output.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer pipe.Close()

	grep.Stdin = pipe
	err = output.Start()
	if err != nil {
		return "", err
	}
	res, _ := grep.Output()

	// format the CPUArchitecture result, the raw "uarch synth" string is like this:
	// (uarch synth) = <vendor> <uarch> {<family>}, <phys>
	// example 1: "(uarch synth) = Intel Sapphire Rapids {Golden Cove}, Intel 7"
	// example 2: "(uarch synth) = AMD Zen 2, 7nm", here the <family> info is missing.
	uarchSection := strings.Split(string(res), "=")
	if len(uarchSection) != 2 {
		return "", fmt.Errorf("cpuid grep output is unexpected")
	}
	// get the string contains only vendor/uarch/family info
	// example 1: "Intel Sapphire Rapids {Golden Cove}"
	// example 2: "AMD Zen 2"
	vendorUarchFamily := strings.Split(strings.TrimSpace(uarchSection[1]), ",")[0]

	// remove the family info if necessary
	var vendorUarch string
	if strings.Contains(vendorUarchFamily, "{") {
		vendorUarch = strings.TrimSpace(strings.Split(vendorUarchFamily, "{")[0])
	} else {
		vendorUarch = vendorUarchFamily
	}

	// get the uarch finally, e.g. "Sapphire Rapids", "Zen 2".
	start := strings.Index(vendorUarch, " ") + 1
	uarch := vendorUarch[start:]

	if err = output.Wait(); err != nil {
		fmt.Printf("cpuid command is not properly completed: %s", err)
	}

	return uarch, err
}

func isFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func main() {
	// init stuffs
	flag.Parse()
	fmt.Println("Dump flag parameters value...")
	fmt.Printf("gen-env:%t\ngen-power:%t\nsampleCount:%d\nsampleDuration:%d\n",
		*genEnv, *genPower, *sampleCount, *sampleDuration)
	cpu, err := ghw.CPU()
	if err != nil {
		fmt.Printf("get cpu info error: %s\n", err)
		return
	}
	pkgNum = len(cpu.Processors)
	samplePowers = make([]NodeComponentsPower, *sampleCount)
	components.InitPowerImpl()
	powerImpl := &source.PowerSysfs{}
	pkgMax, _ = powerImpl.GetMaxEnergyRangeFromPackage()
	coreMax, _ = powerImpl.GetMaxEnergyRangeFromCore()
	uncoreMax, _ = powerImpl.GetMaxEnergyRangeFromUncore()
	dramMax, _ = powerImpl.GetMaxEnergyRangeFromDram()

	platform.InitPowerImpl()

	csvFilePath := filepath.Join(resultDirPath, "power.csv")
	if !isFileExists(csvFilePath) {
		columnHeaders := []string{"Pkg", "Core", "Uncore", "Dram"}
		csvFile, e := os.Create(csvFilePath)
		if e != nil {
			fmt.Printf("failed to create power csv file: %s", e)
			return
		}
		defer csvFile.Close()
		writer := csv.NewWriter(csvFile)
		if err := writer.Write(columnHeaders); err != nil {
			fmt.Printf("error writing column header to csv: %s\n", err)
			return
		}
		writer.Flush()
	}

	if *genEnv {
		//TODO: Currently only support X86 BareMetal platform validation
		cpuArch, err := getX86Architecture()
		if err != nil {
			fmt.Printf("getX86Architecture failed\n")
			return
		}
		var (
			raplEnable       bool
			raplPkgEnable    bool
			raplCoreEnable   bool
			raplUncoreEnable bool
			raplDramEnable   bool
			hmcEnable        bool
			redfishEnable    bool
			acpiEnable       bool
		)
		if components.IsSystemCollectionSupported() {
			raplEnable = true
			if _, err = components.GetAbsEnergyFromPackage(); err == nil {
				raplPkgEnable = true
			}
			if _, err = components.GetAbsEnergyFromCore(); err == nil {
				raplCoreEnable = true
			}
			if _, err = components.GetAbsEnergyFromUncore(); err == nil {
				raplUncoreEnable = true
			}
			if _, err = components.GetAbsEnergyFromDram(); err == nil {
				raplDramEnable = true
			}
		}
		if platform.IsSystemCollectionSupported() {
			powerSource := platform.GetSourceName()
			switch powerSource {
			case "hmc":
				hmcEnable = true
			case "redfish":
				redfishEnable = true
			case "acpi":
				acpiEnable = true
			default:
				fmt.Printf("Unexpected power source: %s\n", powerSource)
			}
		}
		str := fmt.Sprintf("CPU_ARCH=%s\n"+
			"RAPL_ENABLED=%t\n"+
			"RAPL_PKG_ENABLED=%t\n"+
			"RAPL_CORE_ENABLED=%t\n"+
			"RAPL_UNCORE_ENABLED=%t\n"+
			"RAPL_DRAM_ENABLED=%t\n"+
			"HMC_ENABLED=%t\n"+
			"REDFISH_ENABLED=%t\n"+
			"ACPI_ENABLED=%t\n", cpuArch, raplEnable,
			raplPkgEnable, raplCoreEnable,
			raplUncoreEnable, raplDramEnable,
			hmcEnable, redfishEnable, acpiEnable)

		envFilePath := filepath.Join(resultDirPath, "platform-validation.env")
		e := os.WriteFile(envFilePath, []byte(str), 0666)
		if e != nil {
			fmt.Printf("failed to write env file: %s\n", e)
			return
		}
	}

	if *genPower {
		wg := sync.WaitGroup{}
		updateNodePower(&wg)
		wg.Wait()

		pkg := strconv.FormatFloat(meanPower.pkgPower, 'f', 3, 64)
		core := strconv.FormatFloat(meanPower.corePower, 'f', 3, 64)
		uncore := strconv.FormatFloat(meanPower.uncorePower, 'f', 3, 64)
		dram := strconv.FormatFloat(meanPower.dramPower, 'f', 3, 64)

		var outputData []string
		outputData = append(outputData, pkg, core, uncore, dram)

		// open power csv file
		csvFile, err := os.OpenFile(csvFilePath, os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			fmt.Printf("failed to open power csv file: %s\n", err)
			return
		}
		defer csvFile.Close()

		writer := csv.NewWriter(csvFile)

		// store the result data to csv file
		err = writer.Write(outputData)
		if err != nil {
			fmt.Printf("failed to write csv power data :%s \n", err)
			return
		}

		writer.Flush()
		if err := writer.Error(); err != nil {
			fmt.Printf("failed to flush data :%s \n", err)
			return
		}
	}
}
