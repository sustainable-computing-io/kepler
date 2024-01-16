/*
Copyright 2022.

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
	"testing"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
)

var init1Status = []corev1.ContainerStatus{
	{
		ContainerID: "a1",
	},
}

var container1Status = []corev1.ContainerStatus{
	{
		ContainerID: "a2",
	},
	{
		ContainerID: "c1",
	},
}

var eph1Status = []corev1.ContainerStatus{
	{
		ContainerID: "a3",
	},
}

var init2Status = []corev1.ContainerStatus{
	{
		ContainerID: "b1",
	},
	{
		ContainerID: "c2",
	},
}

var container2Status = []corev1.ContainerStatus{
	{
		ContainerID: "b2",
	},
}

var eph2Status = []corev1.ContainerStatus{
	{
		ContainerID: "b3",
	},
	{
		ContainerID: "c3",
	},
}

var normalPods = []corev1.Pod{
	{
		Status: corev1.PodStatus{
			InitContainerStatuses:      init1Status,
			ContainerStatuses:          container1Status,
			EphemeralContainerStatuses: eph1Status,
		},
	},
	{
		Status: corev1.PodStatus{
			InitContainerStatuses:      init2Status,
			ContainerStatuses:          container2Status,
			EphemeralContainerStatuses: eph2Status,
		},
	},
}

var results = map[string]bool{
	"a1": true,
	"a2": true,
	"a3": true,
	"b1": true,
	"b2": true,
	"b3": true,
	"c1": true,
	"c2": true,
	"c3": true,
}

func TestGetAliveContainers(t *testing.T) {
	g := NewWithT(t)

	var testcases = []struct {
		name          string
		pods          []corev1.Pod
		expectErr     bool
		expectResults map[string]bool
	}{
		{
			name:          "test normal status",
			pods:          normalPods,
			expectErr:     false,
			expectResults: results,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			res := getAliveContainers(&testcase.pods)
			g.Expect(res).To(Equal(testcase.expectResults))
		})
	}
}
func TestExtractPodContainerIDfromPathWithCgroup(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		cgroup         int
		expectedResult string
	}{
		{
			name:           "valid path with cgroup 1",
			path:           "1:name=systemd:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podd0511cd2_29d2_4215_be0f_f77bc0609d99.slice/crio-f93ee491b8ed2680d5a909eb098b14a9430173b57ca1c4efedd8768566d67e8e.scope",
			cgroup:         1,
			expectedResult: "f93ee491b8ed2680d5a909eb098b14a9430173b57ca1c4efedd8768566d67e8e",
		},
		{
			name:           "valid path with cgroup 2",
			path:           "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod2c9f8a79_5391_454b_88cb_86190881cb96.slice/crio-a09343ca97901516c25036e2b954421254f8c68b384b536064e8999f0c4ed18d.scope",
			cgroup:         2,
			expectedResult: "a09343ca97901516c25036e2b954421254f8c68b384b536064e8999f0c4ed18d",
		},
		{
			name:           "valid path with cgroup 1",
			path:           "13:hugetlb:/system.slice/docker-2fa3e04b676df750842faf017052dd37ea0cc5bc7259a3487a1718c7fe100c94.scope",
			cgroup:         1,
			expectedResult: "2fa3e04b676df750842faf017052dd37ea0cc5bc7259a3487a1718c7fe100c94",
		},
		{
			name:           "valid path with cgroup kubelet",
			path:           "kubelet/kubepods/besteffort/podbdd4097d-6795-404e-9bd8-6a1383386198/c79788e0da15a6597263eb2b9c51d05dd1a9a1d08c53c1161dc8c45d2dac6b38",
			cgroup:         1,
			expectedResult: "c79788e0da15a6597263eb2b9c51d05dd1a9a1d08c53c1161dc8c45d2dac6b38",
		},
		{
			name:           "valid path with cgroup systemd",
			path:           "/sys/fs/cgroup/systemd/system.slice/containerd.service/kubepods-burstable-poda3b200c9_db51_40b4_9d2d_53f8fdf80d7f.slice:cri-containerd:286b15051ec43375190802e1f40562536980a8fd97e75bb89c7f2eec6f995f17",
			cgroup:         1,
			expectedResult: "286b15051ec43375190802e1f40562536980a8fd97e75bb89c7f2eec6f995f17",
		},
		{
			name:           "valid path with cgroup 13 and memory",
			path:           "13:memory:/system.slice/containerd.service/kubepods-besteffort-pod0043435f_1854_4327_b76b_730f681a781d.slice:cri-containerd:01fd96f7ad292b02a8317cde4ecb8c7ef3cc06ffdd113f13410e0837eb2b2a20",
			cgroup:         1,
			expectedResult: "01fd96f7ad292b02a8317cde4ecb8c7ef3cc06ffdd113f13410e0837eb2b2a20",
		},
		{
			name:           "valid path with cgroup 11 and blkio",
			path:           "11:blkio:/kubepods/burstable/podf6adb0af-0855-4bab-b25b-c853f18d0ce2/35b97177dada20362ab90d90ac63cd54e8a41cf87bea34f270631b6da17f4a93",
			cgroup:         1,
			expectedResult: "35b97177dada20362ab90d90ac63cd54e8a41cf87bea34f270631b6da17f4a93",
		},

		// Add more test cases as needed
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, _ := extractPodContainerIDfromPathWithCgroup(test.path, test.cgroup)
			if result != test.expectedResult {
				t.Errorf("Expected result: %s, but got: %s", test.expectedResult, result)
			}
		})
	}
}
