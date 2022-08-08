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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/zcalusic/sysinfo"
)

const (
	CGROUP_ID_MIN_KERNEL_VERSION = 4.18

	// If this file is present, cgroups v2 is enabled on that node.
	cGroupV2Path = "/sys/fs/cgroup/cgroup.controllers"
)

var (
	EnabledEBPFCgroupID = false
)

// EnableEBPFCgroupID enables the eBPF code to collect cgroup id if the system has kernel version > 4.18
func EnableEBPFCgroupID(enabled bool) {
	fmt.Println("config EnabledEBPFCgroupID ", EnabledEBPFCgroupID)
	fmt.Println("config enabled ", enabled)
	fmt.Println("config getKernelVersion ", getKernelVersion())
	if (enabled == true) && (getKernelVersion() >= CGROUP_ID_MIN_KERNEL_VERSION) && (isCGroupV2()) {
		EnabledEBPFCgroupID = true
	}
}

func getKernelVersion() float32 {
	var si sysinfo.SysInfo

	si.GetSysInfo()

	data, err := json.MarshalIndent(&si, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	var result map[string]map[string]string
	json.Unmarshal([]byte(data), &result)
	val, err := strconv.ParseFloat(result["kernel"]["release"][:3], 32)
	if err != nil {
		log.Fatal(err)
	}

	return float32(val)
}

func isCGroupV2() bool {
	_, err := os.Stat(cGroupV2Path)
	if os.IsNotExist(err) {
		return false
	}
	return true
}
