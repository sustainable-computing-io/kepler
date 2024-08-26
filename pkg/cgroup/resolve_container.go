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
	"sync"

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
	procPath    string = "/proc/%d/cgroup"
	cgroupPath  string = "/sys/fs/cgroup"
)

var (
	instance *cache
	once     sync.Once
)

type cache struct {
	// map to cache data to speedup lookups
	containerIDCache           map[uint64]string
	containerIDToContainerInfo map[string]*ContainerInfo
	cGroupIDToPath             map[uint64]string
	byteOrder                  binary.ByteOrder
	mu                         sync.RWMutex
}

func Init() (*[]corev1.Pod, error) {
	pods := []corev1.Pod{}
	return &pods, nil
}

// InitCache creates the singleton Config instance if necessary.
func InitCache() {
	once.Do(func() {
		instance = newCache()
	})
}

func GetCache() *cache {
	return instance
}

// newConfig creates and returns a new Config instance.
func newCache() *cache {
	return &cache{
		containerIDCache:           map[uint64]string{},
		containerIDToContainerInfo: map[string]*ContainerInfo{},
		cGroupIDToPath:             map[uint64]string{},
		byteOrder:                  utils.DetermineHostByteOrder(),
	}
}

func (c *cache) checkContainerID(id string) bool {
	instance.mu.RLock()
	defer instance.mu.RUnlock()

	_, ok := instance.containerIDToContainerInfo[id]
	return ok
}

func (c *cache) setContainerIDToContainerInfo(id string, info *ContainerInfo) {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	instance.containerIDToContainerInfo[id] = info
}

func (c *cache) getContainerInfo(id string) *ContainerInfo {
	instance.mu.RLock()
	defer instance.mu.RUnlock()
	return instance.containerIDToContainerInfo[id]
}

func (c *cache) setContainerIDCache(pid uint64, id string) {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	instance.containerIDCache[pid] = id
}

func (c *cache) getGetContainerIDFromPID(pid uint64) (string, error) {
	instance.mu.RLock()
	if p, ok := instance.containerIDCache[pid]; ok {
		instance.mu.RUnlock()
		return p, nil
	}
	instance.mu.RUnlock()

	var err error
	var path string
	if path, err = getPathFromPID(procPath, pid); err != nil {
		return utils.SystemProcessName, err
	}

	containerID, err := extractPodContainerIDfromPath(path)
	AddContainerIDToCache(pid, containerID)

	return instance.containerIDCache[pid], err
}

func (c *cache) getContainerIDFromcGroupID(cGroupID uint64) (string, error) {
	instance.mu.RLock()
	if id, ok := instance.containerIDCache[cGroupID]; ok {
		instance.mu.RUnlock()
		return id, nil
	}
	instance.mu.RUnlock()

	var err error
	var path string
	if path, err = instance.getPathFromcGroupID(cGroupID); err != nil {
		return utils.SystemProcessName, err
	}

	containerID, err := extractPodContainerIDfromPath(path)
	AddContainerIDToCache(cGroupID, containerID)

	return instance.containerIDCache[cGroupID], err
}

// getPathFromcGroupID uses cgroupfs to get cgroup path from id
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18+ (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
func (c *cache) getPathFromcGroupID(cgroupID uint64) (string, error) {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	if p, ok := instance.cGroupIDToPath[cgroupID]; ok {
		return p, nil
	}

	err := filepath.WalkDir(cgroupPath, func(path string, dentry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dentry.IsDir() {
			return nil
		}

		getCgroupID, err := utils.GetCgroupIDFromPath(instance.byteOrder, path)
		if err != nil {
			return fmt.Errorf("error resolving handle: %v", err)
		}
		instance.cGroupIDToPath[getCgroupID] = path
		return nil
	})

	if err != nil {
		return unknownPath, fmt.Errorf("failed to find cgroup id: %v", err)
	}
	if p, ok := instance.cGroupIDToPath[cgroupID]; ok {
		return p, nil
	}

	instance.cGroupIDToPath[cgroupID] = unknownPath
	return instance.cGroupIDToPath[cgroupID], nil
}

func (c *cache) getAliveContainers(pods *[]corev1.Pod) map[string]bool {
	aliveContainers := make(map[string]bool)
	instance.mu.Lock()
	defer instance.mu.Unlock()

	for i := 0; i < len(*pods); i++ {
		statuses := (*pods)[i].Status.InitContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
			instance.containerIDToContainerInfo[containerID] = &ContainerInfo{
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
			instance.containerIDToContainerInfo[containerID] = &ContainerInfo{
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
			instance.containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: statuses[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
		}
	}
	return aliveContainers
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

	if instance.checkContainerID(containerID) {
		return instance.getContainerInfo(containerID), nil
	} else {
		info.ContainerID = containerID
		instance.setContainerIDToContainerInfo(containerID, info)
	}

	return instance.getContainerInfo(containerID), nil
}

func ParseContainerIDFromPodStatus(containerID string) string {
	regexReplaceContainerIDPrefix := regexp.MustCompile(`.*//`)
	return regexReplaceContainerIDPrefix.ReplaceAllString(containerID, "")
}

func getContainerIDFromPath(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	if cGroupID == 1 && withCGroupID {
		return utils.KernelProcessName, nil
	}
	var err error
	var containerID string
	if withCGroupID {
		containerID, err = instance.getContainerIDFromcGroupID(cGroupID)
	} else {
		containerID, err = instance.getGetContainerIDFromPID(pid)
	}
	return containerID, err
}

// AddContainerIDToCache add the container id to cache using the pid as the key
func AddContainerIDToCache(pid uint64, containerID string) {
	instance.setContainerIDCache(pid, containerID)
}

// GetContainerIDFromPID find the container ID using the process PID
func GetContainerIDFromPID(pid uint64) (string, error) {
	return instance.getGetContainerIDFromPID(pid)
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

func validContainerID(id string) string {
	validContainerIDRegex := regexp.MustCompile("^[a-zA-Z0-9]+$")
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

// GetAliveContainers returns alive pod map
func GetAliveContainers() (map[string]bool, error) {
	podLister := kubelet.KubeletPodLister{}
	pods, err := podLister.ListPods()
	if err != nil {
		return nil, err
	}

	return instance.getAliveContainers(pods), nil
}
