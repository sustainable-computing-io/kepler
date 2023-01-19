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

package cgroup

import (
	"fmt"
	"os"
	"regexp"

	"k8s.io/klog/v2"
)

var (
	processNameRegex     = "^kworker/"
	processFullNameRegex = "^kworker/.*kblockd"
	kblockCache          = make(map[uint64]bool)
)

func isKblockdWorker(comm string, pid uint64) bool {
	found, _ := regexp.MatchString(processNameRegex, comm)
	// if it is kworker, check the /proc/pid/comm to get full name
	if found {
		if _, ok := kblockCache[pid]; ok {
			return true
		}
		commFile := fmt.Sprintf("/proc/%d/comm", int(pid))
		comm, err := os.ReadFile(commFile)
		if err == nil {
			matched, _ := regexp.MatchString(processFullNameRegex, string(comm))
			if matched {
				klog.V(5).Infof("found kblockd worker %v %s", pid, string(comm))
				kblockCache[pid] = true
			}
			return matched
		}
	}
	return false
}
