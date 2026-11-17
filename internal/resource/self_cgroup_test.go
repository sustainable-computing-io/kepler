// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInPrivateCgroupNamespace(t *testing.T) {
	selfID := strings.Repeat("c", 64)

	tt := []struct {
		name     string
		content  string
		expected bool
	}{
		{name: "cgroup v2 private namespace", content: "0::/\n", expected: true},
		{name: "cgroup v1 private namespace", content: "12:memory:/\n1:name=systemd:/\n0::/\n", expected: true},
		{name: "cgroup v2 host namespace has a full kubepods path", content: "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod5d496b2f.slice/cri-containerd-" + selfID + ".scope\n", expected: false},
		{name: "cgroup v1 host namespace", content: "12:memory:/kubepods/burstable/pod5d496b2f-1111-2222-3333-444455556666/" + selfID + "\n1:name=systemd:/\n", expected: false},
		{name: "empty file", content: "", expected: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "cgroup")
			require.NoError(t, os.WriteFile(file, []byte(tc.content), 0o600))

			got, err := InPrivateCgroupNamespace(file)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}

	t.Run("unreadable file", func(t *testing.T) {
		_, err := InPrivateCgroupNamespace(filepath.Join(t.TempDir(), "missing"))
		assert.Error(t, err)
	})
}

// A process at the root of the host cgroup hierarchy (a kernel thread reads 0::/) is only
// kepler's container when kepler itself sits at its namespace root.
func TestHostNamespaceRootProcessStaysRegular(t *testing.T) {
	selfID := strings.Repeat("c", 64)
	hostCgroup := filepath.Join(t.TempDir(), "cgroup")
	content := "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod5d496b2f.slice/cri-containerd-" + selfID + ".scope\n"
	require.NoError(t, os.WriteFile(hostCgroup, []byte(content), 0o600))

	private, err := InPrivateCgroupNamespace(hostCgroup)
	require.NoError(t, err)
	require.False(t, private)

	// main does not configure the self container in this case
	podInformer := &mockPodInformer{}
	informer, err := NewInformer(WithProcReader(&MockProcReader{}), WithPodInformer(podInformer))
	require.NoError(t, err)
	informer.resolveSelfContainerID()

	kworker := &MockProcInfo{}
	kworker.On("PID").Return(77)
	kworker.On("CPUStat").Return(procCPUStat{CPUTime: 1, StartTime: 10}, nil)
	kworker.On("Comm").Return("kworker/0:1", nil).Maybe()
	kworker.On("Executable").Return("", nil).Maybe()
	kworker.On("Cgroups").Return([]cGroup{{Path: "/"}}, nil).Maybe()
	kworker.On("Environ").Return([]string{}, nil).Maybe()
	kworker.On("CmdLine").Return([]string{}, nil).Maybe()

	proc, err := informer.updateProcessCache(kworker)
	require.NoError(t, err)
	assert.Equal(t, RegularProcess, proc.Type)
	assert.Nil(t, proc.Container)
}
