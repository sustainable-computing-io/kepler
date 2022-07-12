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

package pod_lister

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	ioStatFile = "io.stat"
	reIOStat   = "([0-9]+):([0-9]+).rbytes=([0-9]+).wbytes=([0-9]+)" // 8:16 rbytes=58032128 wbytes=0 rios=120 wios=0 dbytes=0 dios=0
)

var (
	reIO = regexp.MustCompile(reIOStat)
)

func ReadAllCgroupIOStat() (uint64, uint64, int, error) {
	return readIOStat(cgroupPath)
}

func ReadCgroupIOStat(cGroupID uint64) (uint64, uint64, int, error) {
	path, err := getPathFromcGroupID(cGroupID)
	if err != nil {
		return 0, 0, 0, err
	}
	if strings.Contains(path, "crio-") {
		return readIOStat(path)
	}
	return 0, 0, 0, fmt.Errorf("no cgroup path found")
}

func readIOStat(cgroupPath string) (uint64, uint64, int, error) {
	rBytes := uint64(0)
	wBytes := uint64(0)
	disks := 0
	path := filepath.Join(cgroupPath, ioStatFile)
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := reIO.FindStringSubmatch(line)
		l := len(matches)
		if l > 4 {
			major := strings.TrimSpace(matches[l-4])
			if isVirtualDisk(major) {
				continue
			}
			//minor := strings.TrimSpace(matches[l-3])
			r := strings.TrimSpace(matches[l-2])
			w := strings.TrimSpace(matches[l-1])
			disks++
			if val, e := strconv.ParseUint(r, 10, 64); e == nil {
				rBytes += val
			}
			if val, e := strconv.ParseUint(w, 10, 64); e == nil {
				wBytes += val
			}
		}
	}
	//fmt.Printf("path %s read %d write %d ", cgroupPath, rBytes, wBytes)
	return rBytes, wBytes, disks, err
}

func isVirtualDisk(major string) bool {
	if major == "253" { // device-mapper
		return true
	}
	//TODO add other virtual device
	return false
}
