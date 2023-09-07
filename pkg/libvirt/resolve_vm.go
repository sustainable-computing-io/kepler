/*
Copyright 2023.

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

package libvirt

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

const (
	libvirtPath string = "/var/run/libvirt/qemu/"
	procPath    string = "/proc/%s/task"
)

func getThreadIDsForPID(pid, extraPath string) []string {
	threadIDs := []string{}
	fullPath := ""

	if procPath != "" {
		fullPath = filepath.Join(extraPath, procPath)
	} else {
		fullPath = procPath
	}

	procDir := fmt.Sprintf(fullPath, pid)
	files, err := ioutil.ReadDir(procDir)
	if err != nil {
		return nil
	}

	for _, file := range files {
		threadIDs = append(threadIDs, file.Name())
	}

	return threadIDs
}

func GetCurrentVMPID(path ...string) (map[string]string, error) {
	pidFiles := make(map[string]string)

	if len(path) == 0 {
		path = []string{libvirtPath, procPath}
	}

	files, err := ioutil.ReadDir(path[0])
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) == ".pid" {
			filePath := filepath.Join(path[0], file.Name())
			content, err := ioutil.ReadFile(filePath)
			if err != nil {
				fmt.Printf("Error reading %s: %v\n", filePath, err)
				continue
			}

			currentPid := string(content)
			currentName := file.Name()

			tid := getThreadIDsForPID(currentPid, path[1])

			for _, currentTid := range tid {
				// Get rid of the ".pid" before storing the name
				pidFiles[currentTid] = currentName[:len(currentName)-4]
			}
		}
	}

	return pidFiles, nil
}
