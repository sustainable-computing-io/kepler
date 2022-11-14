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

package source

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

func getNumCPUs() int {
	data, err := os.ReadFile(cpuInfoPath)
	if err != nil {
		klog.V(2).Infoln(err)
	}
	return strings.Count(string(data), "processor")
}

func getNumPackage() int {
	var numPackage int
	pkgMap := map[int]bool{}
	numCPUs := getNumCPUs()
	for i := 0; i < numCPUs; i++ {
		path := fmt.Sprintf(numPkgPathTemplate, i)
		data, err := os.ReadFile(path)
		if err != nil {
			break
		}

		if id, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if _, exist := pkgMap[id]; !exist {
				numPackage++
				pkgMap[id] = true
			}
		}
	}
	return numPackage
}

func detectEventPaths() {
	numPackage := getNumPackage()
	for i := 0; i < numPackage; i++ {
		packagePath := fmt.Sprintf(packageNamePathTemplate, i)
		data, err := os.ReadFile(packagePath + "name")
		packageName := strings.TrimSpace(string(data))
		if err != nil {
			continue
		}
		eventPaths[packageName] = map[string]string{}
		eventPaths[packageName][packageName] = packagePath
		for j := 0; j < numRAPLEvents; j++ {
			eventNamePath := fmt.Sprintf(eventNamePathTemplate, i, i, j)
			data, err := os.ReadFile(eventNamePath + "name")
			eventName := strings.TrimSpace(string(data))
			if err != nil {
				continue
			}
			eventPaths[packageName][eventName] = eventNamePath
		}
	}
}

func hasEvent(event string) bool {
	for _, subTree := range eventPaths {
		for e := range subTree {
			if e == event {
				return true
			}
		}
	}
	return false
}
