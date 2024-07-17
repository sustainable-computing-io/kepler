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
	"os"
	"regexp"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

const (
	procPath            string = "/proc/%d/cgroup"
	maxCacheSize        int    = 1000
	cacheRemoveElements int    = 100
)

var (
	cacheExist               = map[uint64]string{}
	cacheNotExist            = map[uint64]bool{}
	regexFindContainerIDPath = regexp.MustCompile(`machine-qemu.*\.scope`)
)

func GetVMID(pid uint64) (string, error) {
	fileName := fmt.Sprintf(procPath, pid)
	return getVMID(pid, fileName)
}

func getVMID(pid uint64, fileName string) (string, error) {
	if _, exist := cacheExist[pid]; exist {
		return cacheExist[pid], nil
	}
	if _, exist := cacheNotExist[pid]; exist {
		return "", fmt.Errorf("pid %d is not in a VM", pid)
	}

	// Read the file
	fileContents, err := os.ReadFile(fileName)
	if err != nil {
		addToNotExistCache(pid)
		return "", err
	}

	content := regexFindContainerIDPath.FindAllString(string(fileContents), -1)
	if len(content) == 0 {
		addToNotExistCache(pid)
		return utils.EmptyString, fmt.Errorf("pid %d does not have vm ID", pid)
	}
	vmID := content[0]
	vmID = strings.ReplaceAll(vmID, "\\x2d", "-")
	vmID = strings.ReplaceAll(vmID, ".scope", "")

	addVMIDToCache(pid, vmID)
	return vmID, nil
}

func addVMIDToCache(pid uint64, id string) {
	if len(cacheExist) >= maxCacheSize {
		counter := cacheRemoveElements
		// Remove elemets from the cache
		for k := range cacheExist {
			delete(cacheExist, k)
			if counter == 0 {
				break
			}
			counter--
		}
	}
	cacheExist[pid] = id
}

func addToNotExistCache(pid uint64) {
	if len(cacheNotExist) >= maxCacheSize {
		counter := cacheRemoveElements
		// Remove elemets from the cache
		for k := range cacheNotExist {
			delete(cacheNotExist, k)
			if counter == 0 {
				break
			}
			counter--
		}
	}
	cacheNotExist[pid] = false
}
