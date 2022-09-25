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
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	cGroupIDMinKernelVersion = 4.18

	// If this file is present, cgroups v2 is enabled on that node.
	cGroupV2Path = "/sys/fs/cgroup/cgroup.controllers"
)

type Client interface {
	getUnixName() (unix.Utsname, error)
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

	versionRegex = regexp.MustCompile(`^(\d+)\.(\d+).`)
)

// EnableEBPFCgroupID enables the eBPF code to collect cgroup id if the system has kernel version > 4.18
func EnableEBPFCgroupID(enabled bool) {
	klog.V(1).Infoln("config EnabledEBPFCgroupID enabled: ", enabled)
	klog.V(1).Infoln("config getKernelVersion: ", getKernelVersion(c))
	if (enabled) && (getKernelVersion(c) >= cGroupIDMinKernelVersion) && (isCGroupV2(c)) {
		EnabledEBPFCgroupID = true
	}
	klog.V(1).Infoln("config set EnabledEBPFCgroupID to ", EnabledEBPFCgroupID)
}

func (c config) getUnixName() (unix.Utsname, error) {
	var utsname unix.Utsname
	err := unix.Uname(&utsname)
	return utsname, err
}

func (c config) getCgroupV2File() string {
	return cGroupV2Path
}

func getKernelVersion(c Client) float32 {
	utsname, err := c.getUnixName()

	if err != nil {
		klog.V(4).Infoln("Failed to parse unix name")
		return -1
	}
	// per https://github.com/google/cadvisor/blob/master/machine/info.go#L164
	kv := utsname.Release[:bytes.IndexByte(utsname.Release[:], 0)]

	versionParts := versionRegex.FindStringSubmatch(string(kv))
	if len(versionParts) < 2 {
		klog.V(1).Infof("got invalid release version %q (expected format '4.3-1 or 4.3.2-1')\n", kv)
		return -1
	}
	major, err := strconv.Atoi(versionParts[1])
	if err != nil {
		klog.V(1).Infof("got invalid release major version %q\n", major)
		return -1
	}

	minor, err := strconv.Atoi(versionParts[2])
	if err != nil {
		klog.V(1).Infof("got invalid release minor version %q\n", minor)
		return -1
	}

	v, err := strconv.ParseFloat(fmt.Sprintf("%d.%d", major, minor), 32)
	if err != nil {
		klog.V(1).Infof("parse %d.%d got issue: %v", major, minor, err)
		return -1
	}
	return float32(v)
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
