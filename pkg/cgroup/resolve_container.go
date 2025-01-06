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
)

type cache struct {
	containerIDCache           sync.Map // map[uint64]string
	containerIDToContainerInfo sync.Map // map[string]*ContainerInfo
	cGroupIDToPath             sync.Map // map[uint64]string
	byteOrder                  binary.ByteOrder
}

func Init() (*[]corev1.Pod, error) {
	pods := []corev1.Pod{}
	return &pods, nil
}

// init() Creates the singleton Config instance
func init() {
	instance = newCache()
}

func GetCache() *cache {
	return instance
}

// newConfig creates and returns a new Config instance.
func newCache() *cache {
	return &cache{
		containerIDCache:           sync.Map{},
		containerIDToContainerInfo: sync.Map{},
		cGroupIDToPath:             sync.Map{},
		byteOrder:                  utils.DetermineHostByteOrder(),
	}
}

func (c *cache) hasContainerID(id string) bool {
	_, ok := instance.containerIDToContainerInfo.Load(id)
	return ok
}

func (c *cache) setContainerIDToContainerInfo(id string, info *ContainerInfo) {
	instance.containerIDToContainerInfo.Store(id, info)
}

func (c *cache) getContainerInfo(id string) (ContainerInfo, error) {
	info, ok := instance.containerIDToContainerInfo.Load(id)
	if !ok {
		return ContainerInfo{}, fmt.Errorf("container info not found for id: %s", id)
	}

	containerInfo, ok := info.(*ContainerInfo)
	if !ok {
		return ContainerInfo{}, fmt.Errorf("invalid type stored for id: %s", id)
	}
	return *containerInfo, nil
}

func (c *cache) setContainerIDCache(pid uint64, id string) {
	instance.containerIDCache.Store(pid, id)
}

func (c *cache) getContainerIDFromCache(pid uint64) (string, bool) {
	value, ok := instance.containerIDCache.Load(pid)
	if !ok {
		return "", false
	}
	containerID, ok := value.(string)
	if !ok {
		return "", false
	}
	return containerID, true
}

func (c *cache) getGetContainerIDFromPID(pid uint64) (string, error) {
	containerID, ok := instance.getContainerIDFromCache(pid)
	if ok {
		return containerID, nil
	}

	path, err := getPathFromPID(procPath, pid)
	if err != nil {
		return utils.SystemProcessName, err
	}

	containerID, err = extractPodContainerIDfromPathWithCgroup(path)
	if err != nil {
		return utils.SystemProcessName, err
	}
	AddContainerIDToCache(pid, containerID)

	return containerID, nil
}

func (c *cache) getContainerIDFromcGroupID(cGroupID uint64) (string, error) {
	id, ok := instance.getContainerIDFromCache(cGroupID)
	if ok {
		return id, nil
	}

	path, err := instance.getPathFromcGroupID(cGroupID)
	if err != nil {
		return utils.SystemProcessName, err
	}

	containerID, err := extractPodContainerIDfromPathWithCgroup(path)
	if err != nil {
		return utils.SystemProcessName, err
	}
	AddContainerIDToCache(cGroupID, containerID)

	return containerID, nil
}

func (c *cache) getPathFromcGroupID(cgroupID uint64) (string, error) {
	p, ok := instance.cGroupIDToPath.Load(cgroupID)
	if ok {
		return p.(string), nil
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
			return fmt.Errorf("error resolving handle: %w", err)
		}
		// found a path and load into cache.
		instance.cGroupIDToPath.Store(getCgroupID, path)
		return nil
	})

	// if error is not nil
	if err != nil {
		return unknownPath, fmt.Errorf("failed to find cgroup id: %v", err)
	}
	// if path found and load from cache.
	p, ok = instance.cGroupIDToPath.Load(cgroupID)
	if ok {
		return p.(string), nil
	}
	// if error is nil, but path not found
	// add mapping in cache
	instance.cGroupIDToPath.Store(cgroupID, unknownPath)
	// return
	return unknownPath, nil
}

func (c *cache) getAliveContainers(pods *[]corev1.Pod) map[string]bool {
	aliveContainers := make(map[string]bool)

	for i := 0; i < len(*pods); i++ {
		statuses := (*pods)[i].Status.InitContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
			instance.setContainerIDToContainerInfo(containerID, &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: statuses[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			})
		}
		statuses = (*pods)[i].Status.ContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
			instance.setContainerIDToContainerInfo(containerID, &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: statuses[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			})
		}
		statuses = (*pods)[i].Status.EphemeralContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
			instance.setContainerIDToContainerInfo(containerID, &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: statuses[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			})
		}
	}
	return aliveContainers
}

func GetContainerID(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	info, err := GetContainerInfo(cGroupID, pid, withCGroupID)
	return info.ContainerID, err
}

func GetContainerInfo(cGroupID, pid uint64, withCGroupID bool) (ContainerInfo, error) {
	var containerID string
	name := utils.SystemProcessName
	namespace := utils.SystemProcessNamespace
	if cGroupID == 1 && withCGroupID {
		name = utils.KernelProcessName
		namespace = utils.KernelProcessNamespace
	}

	info := ContainerInfo{
		ContainerID:   name,
		ContainerName: name,
		PodName:       name,
		Namespace:     namespace,
	}

	containerID, err := getContainerIDFromPath(cGroupID, pid, withCGroupID)
	if err != nil {
		return info, err
	}

	if instance.hasContainerID(containerID) {
		return instance.getContainerInfo(containerID)
	}

	info.ContainerID = containerID
	instance.setContainerIDToContainerInfo(containerID, &info)

	return instance.getContainerInfo(containerID)
}

// ParseContainerIDFromPodStatus removes any prefix from the container ID to standardize it
func ParseContainerIDFromPodStatus(containerID string) string {
	regexReplaceContainerIDPrefix := regexp.MustCompile(`.*//`)
	return regexReplaceContainerIDPrefix.ReplaceAllString(containerID, "")
}

// getContainerIDFromPath retrieves the container ID from the cgroup path or PID
func getContainerIDFromPath(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	if cGroupID == 1 && withCGroupID {
		return utils.KernelProcessName, nil
	}
	if withCGroupID {
		return instance.getContainerIDFromcGroupID(cGroupID)
	}
	return instance.getGetContainerIDFromPID(pid)
}

// extractPodContainerIDfromPathWithCgroup extracts the container ID from a cgroup path
func extractPodContainerIDfromPathWithCgroup(path string) (string, error) {
	if path == unknownPath {
		return utils.SystemProcessName, fmt.Errorf("failed to find pod's container id")
	}

	path = strings.TrimSuffix(path, "/container")
	path = strings.TrimSuffix(path, ".scope")

	// Ensure the path is long enough to extract the container ID
	if len(path) < 64 {
		return utils.SystemProcessName, fmt.Errorf("path too short to determine container ID")
	}

	containerID := path[len(path)-64:]
	return validContainerID(containerID), nil
}

// validContainerID validates and returns the container ID if it matches the expected format
func validContainerID(id string) string {
	validContainerIDRegex := regexp.MustCompile("^[a-zA-Z0-9]+$")
	if validContainerIDRegex.MatchString(id) {
		return id
	}
	return utils.SystemProcessName
}

// AddContainerIDToCache adds the container ID to the cache
func AddContainerIDToCache(pid uint64, containerID string) {
	instance.setContainerIDCache(pid, containerID)
}

// GetContainerIDFromPID retrieves the container ID using the process PID
func GetContainerIDFromPID(pid uint64) (string, error) {
	return instance.getGetContainerIDFromPID(pid)
}

// getPathFromPID retrieves the cgroup path from the PID
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
			// Check if the string has ".scope" in it and truncate everything else after ".scope"
			if strings.Contains(line, ".scope") {
				line = strings.Split(line, ".scope")[0] + ".scope"
			}
			return line, nil
		}
	}
	// This process doesn't belong to a pod, return unknown path to avoid future lookups
	return unknownPath, nil
}

// GetAliveContainers returns a map of alive containers from the provided pods
func GetAliveContainers() (map[string]bool, error) {
	podLister := kubelet.KubeletPodLister{}
	pods, err := podLister.ListPods()
	if err != nil {
		return nil, err
	}

	return instance.getAliveContainers(pods), nil
}
