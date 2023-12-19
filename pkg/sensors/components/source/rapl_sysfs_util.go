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
	"strings"
)

func detectEventPaths() {
	for i := 0; i < numPackages; i++ {
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
			if strings.Index(e, event) == 0 {
				return true
			}
		}
	}
	return false
}
