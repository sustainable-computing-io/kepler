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
	"io/ioutil"
	"strconv"
	"strings"
)

func getNumCPUs() int {
	data, err := ioutil.ReadFile(cpuInfoPath)
	if err != nil {
		fmt.Println(err)
	}
	return strings.Count(string(data), "processor")
}

func getNumPackage() int {
	var numPackage int
	pkgMap := map[int]bool{}
	numCPUs := getNumCPUs()
	for i := 0; i < numCPUs; i++ {
		path := fmt.Sprintf(numPkgPathTemplate, i)
		data, err := ioutil.ReadFile(path)
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
		data, err := ioutil.ReadFile(packagePath + "name")
		packageName := strings.TrimSpace(string(data))
		if err != nil {
			continue
		}
		eventPaths[string(packageName)] = map[string]string{}
		eventPaths[string(packageName)][string(packageName)] = packagePath
		for j := 0; j < numRAPLEvents; j++ {
			eventNamePath := fmt.Sprintf(eventNamePathTemplate, i, i, j)
			data, err := ioutil.ReadFile(eventNamePath + "name")
			eventName := strings.TrimSpace(string(data))
			if err != nil {
				continue
			}
			eventPaths[string(packageName)][string(eventName)] = eventNamePath
		}
	}
}

func hasEvent(event string) bool {
	for _, subTree := range eventPaths {
		for e, _ := range subTree {
			if e == event {
				return true
			}
		}
	}
	return false
}
