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

package metric

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/jszwec/csvutil"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"

	"k8s.io/klog/v2"
)

var CPUModelDataPath = "/var/lib/kepler/data/normalized_cpu_arch.csv"

type CPUModelData struct {
	Architecture string `csv:"Architecture"`
}

func getcontainerUintFeatureNames() []string {
	var metrics []string
	// bpf metrics
	metrics = append(metrics, AvailableBPFSWCounters...)
	// counter metric
	metrics = append(metrics, AvailableBPFHWCounters...)
	// cgroup metric
	metrics = append(metrics, AvailableCGroupMetrics...)
	// cgroup kubelet metric
	metrics = append(metrics, AvailableKubeletMetrics...)
	// gpu metric
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		metrics = append(metrics, []string{config.GPUSMUtilization, config.GPUMemUtilization}...)
	}

	klog.V(3).Infof("Available ebpf metrics: %v", AvailableBPFSWCounters)
	klog.V(3).Infof("Available counter metrics: %v", AvailableBPFHWCounters)
	klog.V(3).Infof("Available cgroup metrics from cgroup: %v", AvailableCGroupMetrics)
	klog.V(3).Infof("Available cgroup metrics from kubelet: %v", AvailableKubeletMetrics)

	return metrics
}

func setEnabledMetrics() {
	ContainerFeaturesNames = []string{}

	ContainerUintFeaturesNames = getcontainerUintFeatureNames()
	ContainerFeaturesNames = append(ContainerFeaturesNames, ContainerFloatFeatureNames...)
	ContainerFeaturesNames = append(ContainerFeaturesNames, ContainerUintFeaturesNames...)
	ContainerFeaturesNames = append(ContainerFeaturesNames, blockDeviceLabel)
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
		return arch
	}
	klog.Errorf("getCPUArch failure: %s", err)
	return "unknown"
}

// getX86Architecture() uses "cpuid" tool to detect the current X86 CPU architecture.
// Take Intel Xeon 4th Gen Scalable Processor(Codename: Sapphire Rapids) as example:
// $ cpuid -1 |grep uarch
//
//	(uarch synth) = Intel Sapphire Rapids {Golden Cove}, Intel 7
//
// In this example, the expected return string should be: "Intel Sapphire Rapids".
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

	// format the CPUArchitecture result
	uarch := strings.Split(string(res), "=")
	if len(uarch) != 2 {
		return "", fmt.Errorf("cpuid grep output is unexpected")
	}

	if err = output.Wait(); err != nil {
		klog.Errorf("cpuid command is not properly completed: %s", err)
	}

	return strings.Split(uarch[1], "{")[0], err
}

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

func getCPUArchitecture() (string, error) {
	// check if there is a CPU architecture override
	cpuArchOverride := config.CPUArchOverride
	if len(cpuArchOverride) > 0 {
		klog.V(2).Infof("cpu arch override: %v\n", cpuArchOverride)
		return cpuArchOverride, nil
	}

	var (
		myCPUModel string
		err        error
	)
	if runtime.GOARCH == "amd64" {
		myCPUModel, err = getX86Architecture()
		if err != nil {
			return "", err
		}
	} else if runtime.GOARCH == "s390x" {
		return getS390xArchitecture()
	} else {
		myCPUModel, err = getArm64Architecture()
		if err != nil {
			return "", err
		}
	}
	file, err := os.Open(CPUModelDataPath)
	if err != nil {
		return "", err
	}
	reader := csv.NewReader(file)

	dec, err := csvutil.NewDecoder(reader)
	if err != nil {
		return "", err
	}

	for {
		var p CPUModelData
		if err := dec.Decode(&p); err == io.EOF {
			break
		}
		if strings.Contains(myCPUModel, p.Architecture) {
			return p.Architecture, nil
		}
	}

	return "", fmt.Errorf("no CPU power model found for architecture %s", myCPUModel)
}

func getCPUPackageMap() (cpuPackageMap map[int32]string) {
	cpuPackageMap = make(map[int32]string)
	// check if mapping available
	numCPU := int32(runtime.NumCPU())
	for cpu := int32(0); cpu < numCPU; cpu++ {
		targetFileName := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/topology/physical_package_id", cpu)
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
