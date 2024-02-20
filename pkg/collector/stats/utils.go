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

package stats

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu"

	cpuidv2 "github.com/klauspost/cpuid/v2"
	"gopkg.in/yaml.v3"
)

const (
	CPUModelDataPath = "/var/lib/kepler/data/cpus.yaml"
	CPUPmuNamePath   = "/sys/devices/cpu/caps/pmu_name"
	CPUTopologyPath  = "/sys/devices/system/cpu/cpu%d/topology/physical_package_id"
)

type CPUModelData struct {
	Core     string `yaml:"core"`
	Uarch    string `yaml:"uarch"`
	Family   string `yaml:"family"`
	Model    string `yaml:"model"`
	Stepping string `yaml:"stepping"`
}

type CPUS struct {
	cpusInfo []CPUModelData
}

func InitAvailableParamAndMetrics() {
	AvailableBPFHWCounters = attacher.GetEnabledBPFHWCounters()
	AvailableBPFSWCounters = attacher.GetEnabledBPFSWCounters()
	AvailableCGroupMetrics = []string{
		config.CgroupfsMemory, config.CgroupfsKernelMemory, config.CgroupfsTCPMemory,
		config.CgroupfsCPU, config.CgroupfsSystemCPU, config.CgroupfsUserCPU,
		config.CgroupfsReadIO, config.CgroupfsWriteIO, config.BlockDevicesIO,
	}
	CPUHardwareCounterEnabled = isCounterStatEnabled(attacher.CPUInstructionLabel)
	AvailableAbsEnergyMetrics = []string{
		config.AbsEnergyInCore, config.AbsEnergyInDRAM, config.AbsEnergyInUnCore, config.AbsEnergyInPkg,
		config.AbsEnergyInGPU, config.AbsEnergyInOther, config.AbsEnergyInPlatform,
	}
	AvailableDynEnergyMetrics = []string{
		config.DynEnergyInCore, config.DynEnergyInDRAM, config.DynEnergyInUnCore, config.DynEnergyInPkg,
		config.DynEnergyInGPU, config.DynEnergyInOther, config.DynEnergyInPlatform,
	}
	AvailableIdleEnergyMetrics = []string{
		config.IdleEnergyInCore, config.IdleEnergyInDRAM, config.IdleEnergyInUnCore, config.IdleEnergyInPkg,
		config.IdleEnergyInGPU, config.IdleEnergyInOther, config.IdleEnergyInPlatform,
	}

	// defined in utils to init metrics
	setEnabledProcessMetrics()
}

func getProcessFeatureNames() []string {
	var metrics []string
	// bpf software counter metrics
	metrics = append(metrics, AvailableBPFSWCounters...)
	klog.V(3).Infof("Available ebpf software counters: %v", AvailableBPFSWCounters)

	// bpf hardware counter metrics
	if config.IsHCMetricsEnabled() {
		metrics = append(metrics, AvailableBPFHWCounters...)
		klog.V(3).Infof("Available ebpf hardware counters: %v", AvailableBPFHWCounters)
	}

	// gpu metric
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		gpuMetrics := []string{config.GPUComputeUtilization, config.GPUMemUtilization}
		metrics = append(metrics, gpuMetrics...)
		klog.V(3).Infof("Available GPU metrics: %v", gpuMetrics)
	}

	// cgroup metric are deprecated and will be removed later
	if config.ExposeCgroupMetrics {
		metrics = append(metrics, AvailableCGroupMetrics...)
		klog.V(3).Infof("Available cgroup metrics from cgroup: %v", AvailableCGroupMetrics)
	}
	return metrics
}

func setEnabledProcessMetrics() {
	ProcessMetricNames = []string{}
	ProcessFeaturesNames = getProcessFeatureNames()
}

func isCounterStatEnabled(label string) bool {
	for _, counter := range AvailableBPFHWCounters {
		if counter == label {
			return true
		}
	}
	return false
}

func GetNodeName() string {
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		return nodeName
	}
	nodeName, err := os.Hostname()
	if err != nil {
		klog.Fatalf("could not get the node name: %s", err)
	}
	return nodeName
}

func getCPUArch() string {
	arch, err := getCPUArchitecture()
	if err == nil {
		klog.V(3).Infof("Current CPU architecture: %s", arch)
		return arch
	}
	klog.Errorf("getCPUArch failure: %s", err)
	return "unknown"
}

// getX86Architecture() uses "cpuid" tool to detect the current X86 CPU architecture.
// Per "cpuid" source code, the "uarch" section format in output is as follows:
//
// "   (uarch synth) = <vendor> <uarch> {<family>}, <phys>"
//
// Take Intel Xeon 4th Gen Scalable Processor(Codename: Sapphire Rapids) as example:
// $ cpuid -1 |grep uarch
//
//	(uarch synth) = Intel Sapphire Rapids {Golden Cove}, Intel 7
//
// In this example, the expected return string should be: "Sapphire Rapids".
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
		klog.Errorf("cpuid command start failure: %s", err)
		return "", err
	}
	res, err := grep.Output()
	if err != nil {
		klog.Errorf("grep cpuid command output failure: %s", err)
		return "", err
	}

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
		klog.Errorf("cpuid command is not properly completed: %s", err)
	}

	return uarch, err
}

// TODO: getCPUArchitecture() code logic changes, need check if anything should change here.
func getArm64Architecture() (string, error) {
	output, err := exec.Command("archspec", "cpu").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

func getS390xArchitecture() (string, error) {
	// use lscpu to get CPUArchitecture
	grep := exec.Command("grep", "Machine type:")
	output := exec.Command("lscpu")
	pipe, err := output.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer pipe.Close()

	grep.Stdin = pipe
	err = output.Start()
	if err != nil {
		klog.Errorf("lscpu command start failure: %s", err)
		return "", err
	}
	res, err := grep.Output()
	if err != nil {
		klog.Errorf("grep lscpu command output failure: %s", err)
		return "", err
	}

	// format the CPUArchitecture result
	uarch := strings.Split(string(res), ":")
	if len(uarch) != 2 {
		return "", fmt.Errorf("lscpu grep output is unexpected")
	}

	if err = output.Wait(); err != nil {
		klog.Errorf("lscpu command is not properly completed: %s", err)
	}

	return fmt.Sprintf("zSystems model %s", strings.TrimSpace(uarch[1])), err
}

// There are three options for Kepler to detect CPU microarchitecture:
// 1. Use tools such as 'cpuid', 'archspec', etc, to directly fetch CPU microarchitecture.
//
//  2. Use Golang libraries to detect the CPU's 'family'/'model'/'stepping' information,
//     then use it to fetch the microarchitecture from community's CPU models data file.
//
// 3. Read Linux SYSFS file '/sys/devices/cpu/pmu_name', use the content as uarch.
//
// Option #1 is the first choice in Kepler, if the tools are N/A on platforms,
// getCPUMicroarchitecture() provides option #2 and #3 as the alternative solution.
func getCPUMicroArchitecture(family, model, stepping string) (string, error) {
	yamlBytes, err := os.ReadFile(CPUModelDataPath)
	if err != nil {
		klog.Errorf("failed to read cpus.yaml: %v", err)
		return "", err
	}
	cpus := &CPUS{
		cpusInfo: []CPUModelData{},
	}
	err = yaml.Unmarshal(yamlBytes, &cpus.cpusInfo)
	if err != nil {
		klog.Errorf("failed to parse cpus.yaml: %v", err)
		return "", err
	}

	for _, info := range cpus.cpusInfo {
		// if family matches
		if info.Family == family {
			var reModel *regexp.Regexp
			reModel, err = regexp.Compile(info.Model)
			if err != nil {
				return "", err
			}
			// if model matches
			if reModel.FindString(model) == model {
				// if there is a stepping
				if info.Stepping != "" {
					var reStepping *regexp.Regexp
					reStepping, err = regexp.Compile(info.Stepping)
					if err != nil {
						return "", err
					}
					// if stepping does NOT match
					if reStepping.FindString(stepping) == "" {
						// no match
						continue
					}
				}
				return info.Uarch, nil
			}
		}
	}
	klog.V(3).Infof("CPU match not found for family %s, model %s, stepping %s. Use pmu_name as uarch.", family, model, stepping)
	// fallback to option #3
	return getCPUPmuName()
}

func getCPUPmuName() (pmuName string, err error) {
	var data []byte
	if data, err = os.ReadFile(CPUPmuNamePath); err != nil {
		klog.V(3).Infoln(err)
		return
	}
	pmuName = string(data)
	return
}

func getCPUArchitecture() (string, error) {
	// check if there is a CPU architecture override
	cpuArchOverride := config.CPUArchOverride
	if len(cpuArchOverride) > 0 {
		klog.V(2).Infof("cpu arch override: %v\n", cpuArchOverride)
		return cpuArchOverride, nil
	}

	var (
		myCPUArch string
		err       error
	)
	// get myCPUArch for x86-64 and ARM CPUs, for s390x CPUs, directly return result.
	if runtime.GOARCH == "amd64" {
		myCPUArch, err = getX86Architecture()
	} else if runtime.GOARCH == "s390x" {
		return getS390xArchitecture()
	} else {
		myCPUArch, err = getArm64Architecture()
	}

	if err == nil {
		return myCPUArch, nil
	} else {
		f, m, s := strconv.Itoa(cpuidv2.CPU.Family), strconv.Itoa(cpuidv2.CPU.Model), strconv.Itoa(cpuidv2.CPU.Stepping)
		return getCPUMicroArchitecture(f, m, s)
	}
}

func getCPUPackageMap() (cpuPackageMap map[int32]string) {
	cpuPackageMap = make(map[int32]string)
	// check if mapping available
	numCPU := int32(runtime.NumCPU())
	for cpu := int32(0); cpu < numCPU; cpu++ {
		targetFileName := fmt.Sprintf(CPUTopologyPath, cpu)
		value, err := os.ReadFile(targetFileName)
		if err != nil {
			klog.Errorf("cannot get CPU-Package map: %v", err)
			return
		}
		cpuPackageMap[cpu] = strings.TrimSpace(string(value))
	}
	klog.V(3).Infof("CPU-Package Map: %v\n", cpuPackageMap)
	return
}
