// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerInfoFromCgroups(t *testing.T) {
	type expect struct {
		id      string
		runtime ContainerRuntime
	}

	tt := []struct {
		name    string
		cgroups []string

		expected expect
	}{{
		name: "Docker container with hyphen",
		cgroups: []string{
			"/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		},
		expected: expect{id: "ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437", runtime: DockerRuntime},
	}, {
		name: "Docker container with slash",
		cgroups: []string{
			"/docker/ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		},
		expected: expect{id: "ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437", runtime: DockerRuntime},
	}, {
		name: "CRI-O container",
		cgroups: []string{
			"/crio-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		expected: expect{id: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", runtime: CrioRuntime},
	}, {
		name: "Podman container",
		cgroups: []string{
			"/libpod-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		expected: expect{id: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", runtime: PodmanRuntime},
	}, {
		name: "Containerd container",
		cgroups: []string{
			"/containerd/1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		expected: expect{id: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", runtime: ContainerDRuntime},
	}, {
		name: "Not a container",
		cgroups: []string{
			"/system.slice/ssh.service",
		},
		expected: expect{id: "", runtime: UnknownRuntime},
	}, {
		name: "Multiple cgroups with container",
		cgroups: []string{
			"/system.slice/user.slice",
			"/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		},
		expected: expect{id: "ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437", runtime: DockerRuntime},
	}, {
		name: "Nested containers in same cgroup path (in kind cluster)",
		cgroups: []string{
			"0::/system.slice/docker-fd9d0ea06257a9780827cbc7fd92e3812a54fca26d63e191b73610d5d48b9cbd.scope/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/kubelet-kubepods-besteffort-podeab5a334_93fe_48a8_b139_9e8079c1f163.slice/cri-containerd-99f3a16ea25b7724cb56a4f0c0df1113ad9474fbf5545bead97fd5c7f61c13f4.scope",
		},
		expected: expect{id: "99f3a16ea25b7724cb56a4f0c0df1113ad9474fbf5545bead97fd5c7f61c13f4", runtime: ContainerDRuntime},
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			runtime, id := containerInfoFromCgroupPaths(tc.cgroups)
			assert.Equal(t, tc.expected.id, id)
			assert.Equal(t, tc.expected.runtime, runtime)
		})
	}
}

func TestContainerIDFromPathWithCgroup(t *testing.T) {
	type expect struct {
		id      string
		runtime ContainerRuntime
	}

	tests := []struct {
		name   string
		path   string
		cgroup int

		expected expect
	}{{
		name: "valid path with cgroup 1",
		path: "1:name=systemd:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podd0511cd2_29d2_4215_be0f_f77bc0609d99.slice/crio-f93ee491b8ed2680d5a909eb098b14a9430173b57ca1c4efedd8768566d67e8e.scope",

		expected: expect{id: "f93ee491b8ed2680d5a909eb098b14a9430173b57ca1c4efedd8768566d67e8e", runtime: CrioRuntime},
	}, {
		name: "valid path with cgroup 2",
		path: "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod2c9f8a79_5391_454b_88cb_86190881cb96.slice/crio-a09343ca97901516c25036e2b954421254f8c68b384b536064e8999f0c4ed18d.scope",

		expected: expect{id: "a09343ca97901516c25036e2b954421254f8c68b384b536064e8999f0c4ed18d", runtime: CrioRuntime},
	}, {
		name: "valid path with cgroup 1",
		path: "13:hugetlb:/system.slice/docker-2fa3e04b676df750842faf017052dd37ea0cc5bc7259a3487a1718c7fe100c94.scope",

		expected: expect{id: "2fa3e04b676df750842faf017052dd37ea0cc5bc7259a3487a1718c7fe100c94", runtime: DockerRuntime},
	}, {
		name: "valid path with cgroup kubelet",
		path: "kubelet/kubepods/besteffort/podbdd4097d-6795-404e-9bd8-6a1383386198/c79788e0da15a6597263eb2b9c51d05dd1a9a1d08c53c1161dc8c45d2dac6b38",

		expected: expect{id: "c79788e0da15a6597263eb2b9c51d05dd1a9a1d08c53c1161dc8c45d2dac6b38", runtime: KubePodsRuntime},
	}, {
		name: "valid path with cgroup systemd",
		path: "/sys/fs/cgroup/systemd/system.slice/containerd.service/kubepods-burstable-poda3b200c9_db51_40b4_9d2d_53f8fdf80d7f.slice:cri-containerd:286b15051ec43375190802e1f40562536980a8fd97e75bb89c7f2eec6f995f17",

		expected: expect{id: "286b15051ec43375190802e1f40562536980a8fd97e75bb89c7f2eec6f995f17", runtime: ContainerDRuntime},
	}, {
		name: "valid path with cgroup 13 and memory",
		path: "13:memory:/system.slice/containerd.service/kubepods-besteffort-pod0043435f_1854_4327_b76b_730f681a781d.slice:cri-containerd:01fd96f7ad292b02a8317cde4ecb8c7ef3cc06ffdd113f13410e0837eb2b2a20",

		expected: expect{id: "01fd96f7ad292b02a8317cde4ecb8c7ef3cc06ffdd113f13410e0837eb2b2a20", runtime: ContainerDRuntime},
	}, {
		name: "valid path with cgroup 11 and blkio",
		path: "11:blkio:/kubepods/burstable/podf6adb0af-0855-4bab-b25b-c853f18d0ce2/35b97177dada20362ab90d90ac63cd54e8a41cf87bea34f270631b6da17f4a93",

		expected: expect{id: "35b97177dada20362ab90d90ac63cd54e8a41cf87bea34f270631b6da17f4a93", runtime: KubePodsRuntime},
	}, {
		name: "podman rootless container",
		path: "0::/user.slice/user-1000.slice/user@1000.service/user.slice/libpod-3f05ee050f82c0145f1d88c94269c39dff0f07dbf8bba20aafd54b3a75dcaecc.scope/container",

		expected: expect{id: "3f05ee050f82c0145f1d88c94269c39dff0f07dbf8bba20aafd54b3a75dcaecc", runtime: PodmanRuntime},
	}, {
		name: "podman rootful container",
		path: "0::/machine.slice/libpod-06dc5f321aad8726aa26559f16ec203bc099245bc44894b14a89fc02b022d1d5.scope/container",

		expected: expect{id: "06dc5f321aad8726aa26559f16ec203bc099245bc44894b14a89fc02b022d1d5", runtime: PodmanRuntime},
	}, {
		name:     "podman-libpod",
		path:     "0::/machine.slice/libpod-8e363eb2287da4ccc9f52ffc5de11252ac5fe707e3ddb917a3c0bdf9bb64165b.scope",
		expected: expect{id: "8e363eb2287da4ccc9f52ffc5de11252ac5fe707e3ddb917a3c0bdf9bb64165b", runtime: PodmanRuntime},
	}, {
		name: "podman quadlet",
		path: "0::/system.slice/kepler.service/libpod-payload-8e363eb2287da4ccc9f52ffc5de11252ac5fe707e3ddb917a3c0bdf9bb64165b",

		expected: expect{id: "8e363eb2287da4ccc9f52ffc5de11252ac5fe707e3ddb917a3c0bdf9bb64165b", runtime: PodmanRuntime},
	}, {
		name: "kind containerd",
		path: "0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-burstable.slice/kubelet-kubepods-burstable-pod3cae2e45_052c_4b11_80d3_4d7b2d2d3464.slice/cri-containerd-2b180104511194aab36fd295d3e217439f3ddb5bc88277f37b4952abee85c40e.scope",

		expected: expect{id: "2b180104511194aab36fd295d3e217439f3ddb5bc88277f37b4952abee85c40e", runtime: ContainerDRuntime},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt, id := containerInfoFromCgroupPaths([]string{test.path})
			assert.Equal(t, test.expected.id, id)
			assert.Equal(t, test.expected.runtime, rt)
		})
	}
}

func TestContainerNameFromEnv(t *testing.T) {
	tt := []struct {
		name         string
		env          []string
		expectedName string
	}{{
		name: "Basic metadata",
		env: []string{
			"CONTAINER_NAME=test-container",
		},
		expectedName: "test-container",
	}, {
		name: "Hostname as container name",
		env: []string{
			"HOSTNAME=test-pod-abcd",
		},
		expectedName: "test-pod-abcd",
	}, {
		name: "Environment with malformed entries",
		env: []string{
			"CONTAINER_NAME=test-container",
			"MALFORMED_ENTRY", // No equals sign
		},
		expectedName: "test-container",
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := containerNameFromEnv(tc.env)
			assert.Equal(t, tc.expectedName, got)
		})
	}
}

func TestContainerNameFromCmdline(t *testing.T) {
	tt := []struct {
		name         string
		cmdline      []string
		expectedName string
	}{{
		name:         "Container with --name=value flag",
		cmdline:      []string{"/bin/containerd", "--name=test-container"},
		expectedName: "test-container",
	}, {
		name:         "Container with --name value flags (separate arguments)",
		cmdline:      []string{"docker", "run", "--name", "my-prom", "prom/prometheus"},
		expectedName: "my-prom",
	}, {
		name:         "Container with --name value at end (edge case)",
		cmdline:      []string{"docker", "run", "--name", "my-container"},
		expectedName: "my-container",
	}, {
		name:         "Container with --name flag but missing value",
		cmdline:      []string{"docker", "run", "--name"},
		expectedName: "",
	}, {
		name:         "docker-containerd-shim with name at position 3",
		cmdline:      []string{"/usr/bin/docker-containerd-shim", "arg1", "arg2", "test-container-name"},
		expectedName: "test-container-name",
	}, {
		name:         "containerd-shim with name at position 3",
		cmdline:      []string{"/usr/bin/containerd-shim", "arg1", "arg2", "test-container-name"},
		expectedName: "test-container-name",
	}, {
		name:         "No matching pattern",
		cmdline:      []string{"/bin/bash", "arg1", "arg2"},
		expectedName: "",
	}, {
		name:         "Empty cmdline",
		cmdline:      []string{},
		expectedName: "",
	}, {
		name:         "Complex docker command with --name value",
		cmdline:      []string{"docker", "run", "-it", "--rm", "--entrypoint", "/bin/sh", "--name", "my-prom", "docker.io/prom/prometheus"},
		expectedName: "my-prom",
	}, {
		name:         "Complex docker command with --name=value",
		cmdline:      []string{"docker", "run", "-it", "--rm", "--entrypoint", "/bin/sh", "--name=my-prom", "docker.io/prom/prometheus"},
		expectedName: "my-prom",
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := containerNameFromCmdLine(tc.cmdline)
			assert.Equal(t, tc.expectedName, got)
		})
	}
}

func TestContainerInfoFromProc(t *testing.T) {
	tt := []struct {
		name              string
		cgroupsPath       string
		environ           []string
		cmdline           []string
		environError      error
		cmdlineError      error
		expectedID        string
		expectedRuntime   ContainerRuntime
		expectedName      string
		expectedNamespace string
		expectedPodName   string
		expectError       bool
	}{{
		name:        "Docker container with complete info",
		cgroupsPath: "/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		environ: []string{
			"CONTAINER_NAME=test-container",
			"KUBERNETES_NAMESPACE=default",
			"KUBERNETES_POD_NAME=test-pod",
		},
		cmdline:           []string{"/bin/bash"},
		expectedID:        "ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		expectedRuntime:   DockerRuntime,
		expectedName:      "test-container",
		expectedNamespace: "default",
		expectedPodName:   "test-pod",
		expectError:       false,
	}, {
		name:        "Not a container",
		cgroupsPath: "/system.slice/ssh.service",
		environ:     []string{},
		cmdline:     []string{"/bin/bash"},
		expectedID:  "",
		expectError: false, // Not an error, just not a container
	}, {
		name:            "Error reading environment",
		cgroupsPath:     "/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		environError:    assert.AnError,
		cmdline:         []string{"/bin/bash"},
		expectedID:      "ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		expectedRuntime: DockerRuntime,
		expectError:     false, // Should continue without environment
	}, {
		name:            "Error reading cmdline",
		cgroupsPath:     "/docker-ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		environ:         []string{"CONTAINER_NAME=test-container"},
		cmdlineError:    assert.AnError,
		expectedID:      "ce82d94d69e1fbbc7feeb66930c69e9b96d9f151f594773e5d0e342741d15437",
		expectedRuntime: DockerRuntime,
		expectedName:    "test-container",
		expectError:     false, // Should continue without cmdline
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mockProc := &MockProcInfo{}
			cgroups := []cGroup{{Path: tc.cgroupsPath}}
			mockProc.On("Cgroups").Return(cgroups, nil)
			mockProc.On("Environ").Return(tc.environ, tc.environError)
			mockProc.On("CmdLine").Return(tc.cmdline, tc.cmdlineError)

			container, err := containerInfoFromProc(mockProc)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tc.expectedID == "" {
				assert.Nil(t, container, "Expected no container to be detected")
				return
			}

			require.NotNil(t, container, "Expected container to be detected")
			assert.Equal(t, tc.expectedID, container.ID)
			assert.Equal(t, tc.expectedRuntime, container.Runtime)
			if tc.expectedName != "" {
				assert.Equal(t, tc.expectedName, container.Name)
			}
		})
	}
}

func TestContainerClone(t *testing.T) {
	t.Run("Full container clone", func(t *testing.T) {
		original := &Container{
			ID:           "1234567890ab",
			Name:         "test-container",
			Runtime:      DockerRuntime,
			CPUTimeDelta: 123.45,
		}

		clone := original.Clone()

		// Check that the clone has the same values
		assert.Equal(t, original.ID, clone.ID)
		assert.Equal(t, original.Name, clone.Name)
		assert.Equal(t, original.Runtime, clone.Runtime)
		assert.Equal(t, float64(0), clone.CPUTimeDelta) // CPUTime shouldn't be cloned
	})

	t.Run("Clone nil container", func(t *testing.T) {
		var nilContainer *Container
		nilClone := nilContainer.Clone()
		assert.Nil(t, nilClone, "Cloning nil container should return nil")
	})
}
