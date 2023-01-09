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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	ioStatFile = "io.stat"
	reIOStat   = `(\d+):(\d+).rbytes=(\d+).wbytes=(\d+)` // 8:16 rbytes=58032128 wbytes=0 rios=120 wios=0 dbytes=0 dios=0
)

var (
	reIO = regexp.MustCompile(reIOStat)
)

func ReadAllCgroupIOStat() (rBytes, wBytes uint64, disks int, err error) {
	path := filepath.Join(cgroupPath, ioStatFile)
	return readIOStat(path)
}

func ReadCgroupIOStat(cGroupID, pid uint64) (rBytes, wBytes uint64, disks int, err error) {
	var path string
	if config.EnabledEBPFCgroupID {
		path, err = getPathFromcGroupID(cGroupID)
	} else {
		path, err = getPathFromPID(procPath, pid)
	}

	if err != nil {
		return 0, 0, 0, err
	}
	if strings.Contains(path, "crio") || strings.Contains(path, "docker") || strings.Contains(path, "containerd") {
		p := filepath.Join(path, ioStatFile)
		return readIOStat(p)
	}
	return 0, 0, 0, fmt.Errorf("no cgroup path found")
}

func readIOStat(path string) (rBytes, wBytes uint64, disks int, err error) {
	rBytes = uint64(0)
	wBytes = uint64(0)
	disks = 0
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
	return rBytes, wBytes, disks, err
}

func isVirtualDisk(major string) bool {
	// TODO add other virtual device
	return major == "253"
}
