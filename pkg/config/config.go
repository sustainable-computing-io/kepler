/*
Copyright 2022.

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

package config

import (
	"fmt"
	"os"
	"strconv"
	"bytes"

	"golang.org/x/sys/unix"
)

const (
	cGroupIDMinKernelVersion = 4.18

	// If this file is present, cgroups v2 is enabled on that node.
	cGroupV2Path = "/sys/fs/cgroup/cgroup.controllers"
)

type Client interface {
	getSysInfo() ([]byte, error)
	getCgroupV2File() string
}

type config struct {
}

var c config

var (
	EnabledEBPFCgroupID = false

	EstimatorModel        = "" // auto-select
	EstimatorSelectFilter = "" // no filter
	CoreUsageMetric       = "curr_cpu_cycles"
	DRAMUsageMetric       = "curr_cache_miss"
	UncoreUsageMetric     = ""                // no metric (evenly divided)
	GeneralUsageMetric    = "curr_cpu_cycles" // for uncategorized energy; pkg - core - uncore
)

// EnableEBPFCgroupID enables the eBPF code to collect cgroup id if the system has kernel version > 4.18
func EnableEBPFCgroupID(enabled bool) {
	fmt.Println("config EnabledEBPFCgroupID enabled: ", enabled)
	fmt.Println("config getKernelVersion: ", getKernelVersion(c))
	if (enabled) && (getKernelVersion(c) >= cGroupIDMinKernelVersion) && (isCGroupV2(c)) {
		EnabledEBPFCgroupID = true
	}
	fmt.Println("config set EnabledEBPFCgroupID to ", EnabledEBPFCgroupID)
}

func (c config) getSysInfo() ([]byte, error) {
	var si sysinfo.SysInfo

	si.GetSysInfo()
	return json.MarshalIndent(&si, "", "  ")
}

func (c config) getCgroupV2File() string {
	return cGroupV2Path
}

func getKernelVersion() float32 {
	var utsname unix.Utsname
	err := unix.Uname(&utsname)
	if err != nil {
		fmt.Println("failed to parse unix name")
		return -1
	}
	// per https://github.com/google/cadvisor/blob/master/machine/info.go#L164
	kv := utsname.Release[:bytes.IndexByte(utsname.Release[:], 0)]
	val, err := strconv.ParseFloat(string(kv[:4]), 32)
	if err == nil {
		return float32(val)
	}
	return -1
}

func isCGroupV2(c Client) bool {
	_, err := os.Stat(c.getCgroupV2File())
	return !os.IsNotExist(err)
}

// Get cgroup version, return 1 or 2
func GetCGroupVersion() int {
	if isCGroupV2(c) {
		return 2
	} else {
		return 1
	}
}

func SetEstimatorConfig(modelName, selectFilter string) {
	EstimatorModel = modelName
	EstimatorSelectFilter = selectFilter
}
