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
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/kubelet"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

type ContainerInfo struct {
	ContainerID   string
	ContainerName string
	PodName       string
	Namespace     string
}

const (
	unknownPath string = "unknown"

	procPath   string = "/proc/%d/cgroup"
	cgroupPath string = "/sys/fs/cgroup"
)

var (
	byteOrder binary.ByteOrder         = utils.DetermineHostByteOrder()
	podLister kubelet.KubeletPodLister = kubelet.KubeletPodLister{}

	// map to cache data to speedup lookups
	containerIDCache           = map[uint64]string{}
	containerIDToContainerInfo = map[string]*ContainerInfo{}
	cGroupIDToPath             = map[uint64]string{}

	validContainerIDRegex         = regexp.MustCompile("^[a-zA-Z0-9]+$")
	regexReplaceContainerIDPrefix = regexp.MustCompile(`.*//`)
)

func Init() (*[]corev1.Pod, error) {
	pods := []corev1.Pod{}
	return &pods, nil
}

func GetContainerID(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	info, err := GetContainerInfo(cGroupID, pid, withCGroupID)
	return info.ContainerID, err
}

func GetContainerInfo(cGroupID, pid uint64, withCGroupID bool) (*ContainerInfo, error) {
	var err error
	var containerID string

	name := utils.SystemProcessName
	namespace := utils.SystemProcessNamespace
	if cGroupID == 1 && withCGroupID {
		// some kernel processes have cgroup id equal 1 or 0
		name = utils.KernelProcessName
		namespace = utils.KernelProcessNamespace
	}
	info := &ContainerInfo{
		ContainerID:   name,
		ContainerName: name,
		PodName:       name,
		Namespace:     namespace,
	}

	if containerID, err = getContainerIDFromPath(cGroupID, pid, withCGroupID); err != nil {
		return info, err
	}

	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	} else {
		info.ContainerID = containerID
		containerIDToContainerInfo[containerID] = info
	}
	return containerIDToContainerInfo[containerID], nil
}

func ParseContainerIDFromPodStatus(containerID string) string {
	return regexReplaceContainerIDPrefix.ReplaceAllString(containerID, "")
}

func getContainerIDFromPath(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	if cGroupID == 1 && withCGroupID {
		return utils.KernelProcessName, nil
	}
	var err error
	var containerID string
	if withCGroupID {
		containerID, err = getContainerIDFromcGroupID(cGroupID)
	} else {
		containerID, err = GetContainerIDFromPID(pid)
	}
	return containerID, err
}

// AddContainerIDToCache add the container id to cache using the pid as the key
func AddContainerIDToCache(pid uint64, containerID string) {
	containerIDCache[pid] = containerID
}

// GetContainerIDFromPID find the container ID using the process PID
func GetContainerIDFromPID(pid uint64) (string, error) {
	if p, ok := containerIDCache[pid]; ok {
		return p, nil
	}

	var err error
	var path string
	if path, err = getPathFromPID(procPath, pid); err != nil {
		return utils.SystemProcessName, err
	}

	containerID, err := extractPodContainerIDfromPath(path)
	AddContainerIDToCache(pid, containerID)
	return containerIDCache[pid], err
}

func getPathFromPID(searchPath string, pid uint64) (string, error) {
	path := fmt.Sprintf(searchPath, pid)
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup description file for pid %d: %v", pid, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "pod") || strings.Contains(line, "containerd") || strings.Contains(line, "crio") {
			// check if the string has ".scope" in it and truncate everything else after ".scope"
			if strings.Contains(line, ".scope") {
				line = strings.Split(line, ".scope")[0] + ".scope"
			}
			return line, nil
		}
	}
	// this process doesn't belong to a pod, return unknown path to avoid future lookups
	return unknownPath, nil
}

func getContainerIDFromcGroupID(cGroupID uint64) (string, error) {
	if id, ok := containerIDCache[cGroupID]; ok {
		return id, nil
	}

	var err error
	var path string
	if path, err = getPathFromcGroupID(cGroupID); err != nil {
		return utils.SystemProcessName, err
	}

	containerID, err := extractPodContainerIDfromPath(path)
	AddContainerIDToCache(cGroupID, containerID)
	return containerIDCache[cGroupID], err
}

// getPathFromcGroupID uses cgroupfs to get cgroup path from id
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18+ (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
func getPathFromcGroupID(cgroupID uint64) (string, error) {
	if p, ok := cGroupIDToPath[cgroupID]; ok {
		return p, nil
	}

	err := filepath.WalkDir(cgroupPath, func(path string, dentry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dentry.IsDir() {
			return nil
		}
		getCgroupID, err := utils.GetCgroupIDFromPath(byteOrder, path)
		if err != nil {
			return fmt.Errorf("error resolving handle: %v", err)
		}
		cGroupIDToPath[getCgroupID] = path
		return nil
	})

	if err != nil {
		return unknownPath, fmt.Errorf("failed to find cgroup id: %v", err)
	}
	if p, ok := cGroupIDToPath[cgroupID]; ok {
		return p, nil
	}

	cGroupIDToPath[cgroupID] = unknownPath
	return cGroupIDToPath[cgroupID], nil
}

func validContainerID(id string) string {
	match := validContainerIDRegex.MatchString(id)
	if match {
		return id
	}
	return utils.SystemProcessName
}

// Get containerID from path. cgroup v1 and cgroup v2 will use different regex
func extractPodContainerIDfromPath(path string) (string, error) {
	return extractPodContainerIDfromPathWithCgroup(path)
}

func extractPodContainerIDfromPathWithCgroup(path string) (string, error) {
	if path == unknownPath {
		return utils.SystemProcessName, fmt.Errorf("failed to find pod's container id")
	}

	path = strings.TrimSuffix(path, "/container")
	path = strings.TrimSuffix(path, ".scope")

	// get the last 64 characters of the path
	if len(path) < 64 {
		return utils.SystemProcessName, fmt.Errorf("failed to find pod's container id")
	}
	containerID := path[len(path)-64:]
	return validContainerID(containerID), nil
}

func getAliveContainers(pods *[]corev1.Pod) map[string]bool {
	aliveContainers := make(map[string]bool)

	for i := 0; i < len(*pods); i++ {
		statuses := (*pods)[i].Status.InitContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: statuses[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
		}
		statuses = (*pods)[i].Status.ContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: statuses[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
		}
		statuses = (*pods)[i].Status.EphemeralContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: statuses[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
		}
	}
	return aliveContainers
}

// GetAliveContainers returns alive pod map
func GetAliveContainers() (map[string]bool, error) {
	pods, err := podLister.ListPods()
	if err != nil {
		return nil, err
	}

	return getAliveContainers(pods), nil
}
