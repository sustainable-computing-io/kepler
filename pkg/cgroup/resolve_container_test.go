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
			res, err := getAliveContainers(&testcase.pods)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(res).To(Equal(testcase.expectResults))
		})
	}
}
