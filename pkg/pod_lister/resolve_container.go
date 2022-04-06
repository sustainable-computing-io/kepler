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
	"strings"
)

type ContainerInfo struct {
	PodName       string
	ContainerName string
	Namespace     string
}

const (
	systemProcessName      string = "system_processes"
	systemProcessNamespace string = "system"
	procPath               string = "/proc/%d/cgroup"
)

var (
	pidToContainerIDCache      = map[uint64]string{}
	ContainerIDToContainerInfo = map[string]*ContainerInfo{}
	podLister                  = KubeletPodLister{}
)

func GetPodNameFromPID(pid uint64) (string, error) {
	info, err := getContainerInfoFromPID(pid)
	return info.PodName, err
}

func GetPodNameSpaceFromPID(pid uint64) (string, error) {
	info, err := getContainerInfoFromPID(pid)
	return info.Namespace, err
}

func GetPodContainerNameFromPID(pid uint64) (string, error) {
	info, err := getContainerInfoFromPID(pid)
	return info.ContainerName, err
}

func getContainerInfoFromPID(pid uint64) (*ContainerInfo, error) {
	info := &ContainerInfo{
		PodName:   systemProcessName,
		Namespace: systemProcessNamespace,
	}

	containerID, err := getContainerIDFromPID(pid)
	if err != nil {
		return info, err
	}

	if info, ok := ContainerIDToContainerInfo[containerID]; ok {
		return info, nil
	}

	pods, err := podLister.ListPods()
	if err != nil {
		return info, err
	}

	for _, pod := range *pods {
		info.PodName = pod.Name
		info.Namespace = pod.Namespace
		statuses := pod.Status.ContainerStatuses
		for _, status := range statuses {
			if status.ContainerID == containerID {
				info.ContainerName = status.Name
				return info, nil
			}
		}
		statuses = pod.Status.InitContainerStatuses
		for _, status := range statuses {
			if status.ContainerID == containerID {
				info.ContainerName = status.Name
				fmt.Println(pod.Name, status.Name)
				return info, nil
			}
		}
	}

	return info, fmt.Errorf("could not match containerID: %s to any running pod", containerID)
}

//TODO: test this function in kubernetes, not only openshift
func getContainerIDFromPID(pid uint64) (string, error) {
	if p, ok := pidToContainerIDCache[pid]; ok {
		return p, nil
	}

	path := fmt.Sprintf(procPath, pid)
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open cgroup description file for pid %d: %v", pid, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "-conmon-") || strings.Contains(line, ".service") {
			return "", fmt.Errorf("process pid %d is not in a kubernetes pod", pid)

		} else if strings.Contains(line, "pod-") {
			cgroup := strings.Split(line, "/")
			containerID := cgroup[len(cgroup)-1]
			pidToContainerIDCache[pid] = containerID

		} else if strings.Contains(line, "crio") {
			cgroup := strings.Split(line, "/")
			containerID := cgroup[len(cgroup)-1]
			containerID = strings.Trim(containerID, "crio-")
			containerID = strings.Trim(containerID, ".scope")
			containerID = "cri-o://" + containerID
			pidToContainerIDCache[pid] = containerID
		}
	}
	if p, ok := pidToContainerIDCache[pid]; ok {
		return p, nil
	}
	return "system_process", nil
}
