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
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

type ContainerInfo struct {
	PodName       string
	ContainerName string
	Namespace     string
}

const (
	systemProcessName      string = "system_processes"
	systemProcessNamespace string = "system"
	unknownPath            string = "unknown"

	procPath   string = "/proc/%d/cgroup"
	cgroupPath string = "/sys/fs/cgroup"
)

var (
	byteOrder binary.ByteOrder = determineHostByteOrder()
	podLister KubeletPodLister = KubeletPodLister{}

	//map to cache data to speedup lookups
	containerIDCache           = map[uint64]string{}
	containerIDToContainerInfo = map[string]*ContainerInfo{}
	cGroupIDToPath             = map[uint64]string{}

	//regex to extract container ID from path
	regexFindContainerIDPath          = regexp.MustCompile(`.*-(.*?)\.scope`)
	regexReplaceContainerIDPathPrefix = regexp.MustCompile(`.*-`)
	regexReplaceContainerIDPathSufix  = regexp.MustCompile(`\..*`)
	regexReplaceContainerIDPrefix     = regexp.MustCompile(`.*//`)
)

func init() {
	updateListPodCache("", false)
}

func GetSystemProcessName() string {
	return systemProcessName
}

func GetSystemProcessNamespace() string {
	return systemProcessNamespace
}

func GetPodName(cGroupID uint64, PID uint64) (string, error) {
	info, err := getContainerInfo(cGroupID, PID)
	return info.PodName, err
}

func GetPodNameSpace(cGroupID uint64, PID uint64) (string, error) {
	info, err := getContainerInfo(cGroupID, PID)
	return info.Namespace, err
}

func GetPodContainerName(cGroupID uint64, PID uint64) (string, error) {
	info, err := getContainerInfo(cGroupID, PID)
	return info.ContainerName, err
}

func GetPodMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error) {
	return podLister.ListMetrics()
}

func GetAvailableKubeletMetrics() []string {
	return podLister.GetAvailableMetrics()
}

func getContainerInfo(cGroupID uint64, PID uint64) (*ContainerInfo, error) {
	var err error
	var containerID string
	info := &ContainerInfo{
		PodName:   systemProcessName,
		Namespace: systemProcessNamespace,
	}

	if containerID, err = GetContainerID(cGroupID, PID); err != nil {
		return info, nil
	}

	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	}

	// update cache info and stop loop if container id found
	updateListPodCache(containerID, true)
	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	}

	containerIDToContainerInfo[containerID] = info
	return containerIDToContainerInfo[containerID], nil
}

// updateListPodCache updates cache info with all pods and optionally
// stops the loop when a given container ID is found
func updateListPodCache(targetContainerID string, stopWhenFound bool) {
	pods, err := podLister.ListPods()
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	for _, pod := range *pods {
		statuses := pod.Status.ContainerStatuses
		for _, status := range statuses {
			info := &ContainerInfo{
				PodName:       pod.Name,
				Namespace:     pod.Namespace,
				ContainerName: status.Name,
			}
			containerID := regexReplaceContainerIDPrefix.ReplaceAllString(status.ContainerID, "")
			containerIDToContainerInfo[containerID] = info
			if stopWhenFound && status.ContainerID == targetContainerID {
				return
			}
		}
		statuses = pod.Status.InitContainerStatuses
		for _, status := range statuses {
			info := &ContainerInfo{
				PodName:       pod.Name,
				Namespace:     pod.Namespace,
				ContainerName: status.Name,
			}
			containerID := regexReplaceContainerIDPrefix.ReplaceAllString(status.ContainerID, "")
			containerIDToContainerInfo[containerID] = info
			if stopWhenFound && status.ContainerID == targetContainerID {
				return
			}
		}
	}
}

func GetContainerID(cGroupID uint64, PID uint64) (string, error) {
	var err error
	var containerID string
	if config.EnabledEBPFCgroupID {
		containerID, err = getContainerIDFromcGroupID(cGroupID)
	} else {
		containerID, err = getContainerIDFromPID(PID)
	}
	return containerID, err
}

func getContainerIDFromPID(pid uint64) (string, error) {
	if p, ok := containerIDCache[pid]; ok {
		return p, nil
	}

	var err error
	var path string
	if path, err = getPathFromPID(pid); err != nil {
		return systemProcessName, err
	}

	containerIDCache[pid], err = extractPodContainerIDfromPath(path)
	return containerIDCache[pid], err
}

func getPathFromPID(pid uint64) (string, error) {
	path := fmt.Sprintf(procPath, pid)
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup description file for pid %d: %v", pid, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "pod") || strings.Contains(line, "crio") {
			return line, nil
		}
	}
	return "", fmt.Errorf("could not find cgroup description file for pid %d", pid)
}

func getContainerIDFromcGroupID(cGroupID uint64) (string, error) {
	if id, ok := containerIDCache[cGroupID]; ok {
		return id, nil
	}

	var err error
	var path string
	if path, err = getPathFromcGroupID(cGroupID); err != nil {
		return systemProcessName, err
	}

	containerIDCache[cGroupID], err = extractPodContainerIDfromPath(path)
	return containerIDCache[cGroupID], err
}

// getPathFromcGroupID uses cgroupfs to get cgroup path from id
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18+ (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
func getPathFromcGroupID(cgroupId uint64) (string, error) {
	if p, ok := cGroupIDToPath[cgroupId]; ok {
		return p, nil
	}

	err := filepath.WalkDir(cgroupPath, func(path string, dentry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dentry.IsDir() {
			return nil
		}
		handle, _, err := unix.NameToHandleAt(unix.AT_FDCWD, path, 0)
		if err != nil {
			return fmt.Errorf("error resolving handle: %v", err)
		}
		cGroupIDToPath[byteOrder.Uint64(handle.Bytes())] = path
		return nil
	})

	if err != nil {
		return unknownPath, fmt.Errorf("failed to find cgroup id: %v", err)
	}
	if p, ok := cGroupIDToPath[cgroupId]; ok {
		return p, nil
	}

	cGroupIDToPath[cgroupId] = unknownPath
	return cGroupIDToPath[cgroupId], nil
}

func extractPodContainerIDfromPath(path string) (string, error) {
	if regexFindContainerIDPath.MatchString(path) {
		sub := regexFindContainerIDPath.FindAllString(path, -1)
		for _, element := range sub {
			if strings.Contains(element, "-conmon-") || strings.Contains(element, ".service") {
				return "", fmt.Errorf("process is not in a kubernetes pod")
				//TODO: we need to extend this to include other runtimes
			} else if strings.Contains(element, "crio") || strings.Contains(element, "docker") || strings.Contains(element, "containerd") {
				containerID := regexReplaceContainerIDPathPrefix.ReplaceAllString(element, "")
				containerID = regexReplaceContainerIDPathSufix.ReplaceAllString(containerID, "")
				return containerID, nil
			}
		}
	}
	return systemProcessName, fmt.Errorf("failed to find pod's container id")
}
