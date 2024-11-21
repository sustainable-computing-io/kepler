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
package node

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	cpuidv2 "github.com/klauspost/cpuid/v2"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	cpuModelDataPath = "/var/lib/kepler/data/cpus.yaml"
	cpuPmuNamePath   = "/sys/devices/cpu/caps/pmu_name"
	unknownCPUArch   = "unknown"
)

type nodeInfo struct {
	name                  string
	cpuArchitecture       string
	metadataFeatureNames  []string
	metadataFeatureValues []string
}

type Node interface {
	Name() string
	CPUArchitecture() string
	MetadataFeatureNames() []string
	MetadataFeatureValues() []string
}

type cpuModelData struct {
	Uarch    string `yaml:"uarch"`
	Family   string `yaml:"family"`
	Model    string `yaml:"model"`
	Stepping string `yaml:"stepping"`
}

type cpuInfo struct {
	cpusInfo []cpuModelData
}

func Name() string {
	return nodeName()
}

func CPUArchitecture() string {
	return cpuArch()
}

func MetadataFeatureNames() []string {
	return []string{"cpu_architecture"}
}

func MetadataFeatureValues() []string {
	cpuArchitecture := cpuArch()
	return []string{cpuArchitecture}
}

func NewNodeInfo() Node {
	cpuArchitecture := cpuArch()
	return &nodeInfo{
		name:                  nodeName(),
		cpuArchitecture:       cpuArchitecture,
		metadataFeatureNames:  []string{"cpu_architecture"},
		metadataFeatureValues: []string{cpuArchitecture},
	}
}

func (ni *nodeInfo) Name() string {
	return ni.name
}

func (ni *nodeInfo) CPUArchitecture() string {
	return ni.cpuArchitecture
}

func (ni *nodeInfo) MetadataFeatureNames() []string {
	return ni.metadataFeatureNames
}

func (ni *nodeInfo) MetadataFeatureValues() []string {
	return ni.metadataFeatureValues
}

func nodeName() string {
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		return nodeName
	}
	nodeName, err := os.Hostname()
	if err != nil {
		klog.Fatalf("could not get the node name: %s", err)
	}
	return nodeName
}

func cpuArch() string {
	if arch, err := getCPUArchitecture(); err == nil {
		klog.V(3).Infof("Current CPU architecture: %s", arch)
		return arch
	} else {
		klog.Errorf("getCPUArch failure: %s", err)
		return unknownCPUArch
	}
}

func x86Architecture() (string, error) {
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

	uarchSection := strings.Split(string(res), "=")
	if len(uarchSection) != 2 {
		return "", fmt.Errorf("cpuid grep output is unexpected")
	}
	vendorUarchFamily := strings.Split(strings.TrimSpace(uarchSection[1]), ",")[0]

	var vendorUarch string
	if strings.Contains(vendorUarchFamily, "{") {
		vendorUarch = strings.TrimSpace(strings.Split(vendorUarchFamily, "{")[0])
	} else {
		vendorUarch = vendorUarchFamily
	}

	start := strings.Index(vendorUarch, " ") + 1
	uarch := vendorUarch[start:]

	if err = output.Wait(); err != nil {
		klog.Errorf("cpuid command is not properly completed: %s", err)
	}

	return uarch, err
}

func s390xArchitecture() (string, error) {
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

	uarch := strings.Split(string(res), ":")
	if len(uarch) != 2 {
		return "", fmt.Errorf("lscpu grep output is unexpected")
	}

	if err = output.Wait(); err != nil {
		klog.Errorf("lscpu command is not properly completed: %s", err)
	}

	return fmt.Sprintf("zSystems model %s", strings.TrimSpace(uarch[1])), err
}

func readCPUModelData() ([]byte, error) {
	data, err := os.ReadFile(cpuModelDataPath)
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	klog.Infof("%s not found, reading local cpus.yaml", cpuModelDataPath)
	return os.ReadFile("./data/cpus.yaml")
}

func cpuMicroArchitectureFromModel(yamlBytes []byte, family, model, stepping string) (string, error) {
	cpus := &cpuInfo{
		cpusInfo: []cpuModelData{},
	}
	err := yaml.Unmarshal(yamlBytes, &cpus.cpusInfo)
	if err != nil {
		klog.Errorf("failed to parse cpus.yaml: %v", err)
		return "", err
	}

	for _, info := range cpus.cpusInfo {
		if info.Family == family {
			var reModel *regexp.Regexp
			reModel, err = regexp.Compile(info.Model)
			if err != nil {
				return "", err
			}
			if reModel.FindString(model) == model {
				if info.Stepping != "" {
					var reStepping *regexp.Regexp
					reStepping, err = regexp.Compile(info.Stepping)
					if err != nil {
						return "", err
					}
					if reStepping.FindString(stepping) == "" {
						continue
					}
				}
				return info.Uarch, nil
			}
		}
	}
	klog.V(3).Infof("CPU match not found for family %s, model %s, stepping %s. Use pmu_name as uarch.", family, model, stepping)
	return "unknown", fmt.Errorf("CPU match not found")
}

func cpuMicroArchitecture(family, model, stepping string) (string, error) {
	yamlBytes, err := readCPUModelData()
	if err != nil {
		klog.Errorf("failed to read cpus.yaml: %v", err)
		return "", err
	}
	arch, err := cpuMicroArchitectureFromModel(yamlBytes, family, model, stepping)
	if err == nil {
		return arch, nil
	}
	return cpuPmuName()
}

func cpuPmuName() (string, error) {
	data, err := os.ReadFile(cpuPmuNamePath)
	if err != nil {
		klog.V(3).Infoln(err)
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func getCPUArchitecture() (string, error) {
	cpuArchOverride := config.CPUArchOverride()
	if cpuArchOverride != "" {
		klog.V(2).Infof("cpu arch override: %v\n", cpuArchOverride)
		return cpuArchOverride, nil
	}

	var (
		myCPUArch string
		err       error
	)
	if runtime.GOARCH == "amd64" {
		myCPUArch, err = x86Architecture()
	} else if runtime.GOARCH == "s390x" {
		return s390xArchitecture()
	} else {
		myCPUArch, err = cpuPmuName()
	}

	if err == nil {
		return myCPUArch, nil
	} else {
		f, m, s := strconv.Itoa(cpuidv2.CPU.Family), strconv.Itoa(cpuidv2.CPU.Model), strconv.Itoa(cpuidv2.CPU.Stepping)
		return cpuMicroArchitecture(f, m, s)
	}
}
