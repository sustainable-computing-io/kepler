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
	"time"

	internalapi "k8s.io/cri-api/pkg/apis"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	kuberuntime "k8s.io/kubernetes/pkg/kubelet/cri/remote"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/kubelet"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
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

	defaultTimeout                    = 5 * time.Second
	dockerShimSock             string = "unix:///var/run/dockershim.sock"
	containerdSock             string = "unix:///run/containerd/containerd.sock"
	crioSock                   string = "unix:///run/crio/crio.sock"
	containerLabelName         string = "io.kubernetes.container.name"
	containerLabelPodName      string = "io.kubernetes.pod.name"
	containerLabelPodNamespace string = "io.kubernetes.pod.namespace"
)

var (
	byteOrder binary.ByteOrder         = utils.DetermineHostByteOrder()
	podLister kubelet.KubeletPodLister = kubelet.KubeletPodLister{}

	// map to cache data to speedup lookups
	containerIDCache           = map[uint64]string{}
	containerIDToContainerInfo = map[string]*ContainerInfo{}
	cGroupIDToPath             = map[uint64]string{}

	// regex to extract container ID from path
	regexFindContainerIDPath          = regexp.MustCompile(`.*-(.*?)\.scope`)
	regexReplaceContainerIDPathPrefix = regexp.MustCompile(`.*-`)
	// some platforms (e.g. RHEL) have different cgroup path
	regexFindContainerIDPath2 = regexp.MustCompile(`[^:]*$`)

	regexReplaceContainerIDPathSufix = regexp.MustCompile(`\..*`)
	regexReplaceContainerIDPrefix    = regexp.MustCompile(`.*//`)

	containerRuntime internalapi.RuntimeService
)

func init() {
	var err error
	defer func() {
		if r := recover(); r != nil {
			klog.V(10).Infoln(err)
		}
	}() // disable the anoying error printing when trying different runtimes
	var endPoint string
	for _, endPoint = range []string{dockerShimSock, containerdSock, crioSock} {
		r, err := kuberuntime.NewRemoteRuntimeService(endPoint, defaultTimeout)
		if err != nil {
			klog.V(5).Infof("Creation of container runtime %s failed: %v", endPoint, err)
			continue
		}
		_, err = r.Status()
		if err != nil {
			klog.V(5).Infof("Status from container runtime service failed: %v", err)
			continue
		}

		klog.Infof("Successfully connected to runtime %s", endPoint)
		containerRuntime = r
		break
	}
}

func GetPodName(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	info, err := getContainerInfo(cGroupID, pid, withCGroupID)
	return info.PodName, err
}

func GetPodNameSpace(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	info, err := getContainerInfo(cGroupID, pid, withCGroupID)
	return info.Namespace, err
}

func GetContainerName(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	info, err := getContainerInfo(cGroupID, pid, withCGroupID)
	return info.ContainerName, err
}

func GetContainerID(cGroupID, pid uint64, withCGroupID bool) (string, error) {
	info, err := getContainerInfo(cGroupID, pid, withCGroupID)
	return info.ContainerID, err
}

func GetContainerMetrics() (containerCPU, containerMem map[string]float64, nodeCPU, nodeMem float64, retErr error) {
	return podLister.ListMetrics()
}

func GetAvailableKubeletMetrics() []string {
	return podLister.GetAvailableMetrics()
}

func getContainerInfo(cGroupID, pid uint64, withCGroupID bool) (*ContainerInfo, error) {
	var err error
	var containerID string
	info := &ContainerInfo{
		ContainerID:   utils.SystemProcessName,
		ContainerName: utils.SystemProcessName,
		PodName:       utils.SystemProcessName,
		Namespace:     utils.SystemProcessNamespace,
	}
	// pid 0 means kernel processes
	if pid == 0 {
		return info, nil
	}
	if containerID, err = getContainerIDFromPath(cGroupID, pid, withCGroupID); err != nil {
		return info, err
	}
	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	}
	if cinfo, err := getContainerInfoFromRuntime(containerID); (err == nil) && (cinfo != nil) {
		return cinfo, nil
	}
	if _, ok := containerIDToContainerInfo[containerID]; !ok {
		containerIDToContainerInfo[containerID] = info
	}
	return info, nil
}

func getContainerInfoFromRuntime(containerID string) (*ContainerInfo, error) {
	var err error
	var cstatus *v1.ContainerStats
	if containerID == "unknown" {
		return nil, fmt.Errorf("could not find the container")
	}
	if cstatus, err = containerRuntime.ContainerStats(containerID); err == nil {
		containerIDToContainerInfo[containerID] = &ContainerInfo{
			ContainerID:   containerID,
			ContainerName: cstatus.Attributes.Labels[containerLabelName],
			PodName:       cstatus.Attributes.Labels[containerLabelPodName],
			Namespace:     cstatus.Attributes.Labels[containerLabelPodNamespace],
		}
		return containerIDToContainerInfo[containerID], nil
	}
	return nil, fmt.Errorf("could not find the container")
}

// ListPodsAndUpdateCache updates cache info with all pods and optionally
func ListPodsAndUpdateCache() (*[]corev1.Pod, error) {
	pods, err := podLister.ListPods()
	if err != nil {
		klog.V(4).Infof("%v", err)
		return pods, err
	}
	for i := 0; i < len(*pods); i++ {
		containers := (*pods)[i].Status.ContainerStatuses
		for j := 0; j < len(containers); j++ {
			containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: containers[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
		}
		containers = (*pods)[i].Status.InitContainerStatuses
		for j := 0; j < len(containers); j++ {
			containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: containers[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
		}
		containers = (*pods)[i].Status.EphemeralContainerStatuses
		for j := 0; j < len(containers); j++ {
			containerID := ParseContainerIDFromPodStatus(containers[j].ContainerID)
			containerIDToContainerInfo[containerID] = &ContainerInfo{
				ContainerID:   containerID,
				ContainerName: containers[j].Name,
				PodName:       (*pods)[i].Name,
				Namespace:     (*pods)[i].Namespace,
			}
		}
	}
	return pods, nil
}

func ParseContainerIDFromPodStatus(containerID string) string {
	return regexReplaceContainerIDPrefix.ReplaceAllString(containerID, "")
}

func getContainerIDFromPath(cGroupID, pid uint64, withCGroupID bool) (string, error) {
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
			return line, nil
		}
	}
	return "", fmt.Errorf("could not find cgroup description entry for pid %d", pid)
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

// Get containerID from path. cgroup v1 and cgroup v2 will use different regex
func extractPodContainerIDfromPath(path string) (string, error) {
	cgroup := config.GetCGroupVersion()
	if regexFindContainerIDPath.MatchString(path) {
		sub := regexFindContainerIDPath.FindAllString(path, -1)
		for _, element := range sub {
			if cgroup == 2 && (strings.Contains(element, "-conmon-") || strings.Contains(element, ".service")) {
				return "", fmt.Errorf("process is not in a kubernetes pod")
				// TODO: we need to extend this to include other runtimes
			} else if strings.Contains(element, "crio") || strings.Contains(element, "docker") || strings.Contains(element, "containerd") {
				containerID := regexReplaceContainerIDPathPrefix.ReplaceAllString(element, "")
				containerID = regexReplaceContainerIDPathSufix.ReplaceAllString(containerID, "")
				return containerID, nil
			}
		}
	}
	// as some platforms (e.g. RHEL) have a different cgroup path, if the cgroup path has information from a pod and we can't get the container id
	// with the previous regex, we'll try to get it using a different approach
	if regexFindContainerIDPath2.MatchString(path) {
		sub := regexFindContainerIDPath2.FindAllString(path, -1)
		for _, containerID := range sub {
			return containerID, nil
		}
	}
	return utils.SystemProcessName, fmt.Errorf("failed to find pod's container id")
}

func getAliveContainers(pods *[]corev1.Pod) (map[string]bool, error) {
	aliveContainers := make(map[string]bool)

	for i := 0; i < len(*pods); i++ {
		statuses := (*pods)[i].Status.InitContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
		}
		statuses = (*pods)[i].Status.ContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
		}
		statuses = (*pods)[i].Status.EphemeralContainerStatuses
		for j := 0; j < len(statuses); j++ {
			containerID := ParseContainerIDFromPodStatus(statuses[j].ContainerID)
			aliveContainers[containerID] = true
		}
	}
	return aliveContainers, nil
}

// GetAliveContainers returns alive pod map
func GetAliveContainers() (map[string]bool, error) {
	pods, err := podLister.ListPods()
	if err != nil {
		return nil, err
	}

	return getAliveContainers(pods)
}
