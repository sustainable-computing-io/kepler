//go:build linux
// +build linux

/*
Copyright 2023.

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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/containerd/cgroups"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/opencontainers/runtime-spec/specs-go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	defaultDirPerm = 0o755
)

// defaultFilePerm is a var so that the test framework can change the filemode
// of all files created when the tests are running.  The difference between the
// tests and real world use is that files like "cgroup.procs" will exist when writing
// to a read cgroup filesystem and do not exist prior when running in the tests.
// this is set to a non 0 value in the test code
var defaultFilePerm = os.FileMode(defaultDirPerm)

type mockCgroup struct {
	root       string
	subsystems []cgroup1.Subsystem
}

func (m *mockCgroup) delete() error {
	return os.RemoveAll(m.root)
}

func (m *mockCgroup) hierarchy() ([]cgroup1.Subsystem, error) {
	return m.subsystems, nil
}

// this functions is a copy from the containerd cgroup lib test
func defaults(root string) ([]cgroup1.Subsystem, error) {
	h, err := cgroup1.NewHugetlb(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	s := []cgroup1.Subsystem{
		cgroup1.NewNamed(root, "systemd"),
		cgroup1.NewFreezer(root),
		cgroup1.NewPids(root),
		cgroup1.NewNetCls(root),
		cgroup1.NewNetPrio(root),
		cgroup1.NewPerfEvent(root),
		cgroup1.NewCpuset(root),
		cgroup1.NewCpu(root),
		cgroup1.NewCpuacct(root),
		cgroup1.NewMemory(root),
		cgroup1.NewBlkio(root),
		cgroup1.NewRdma(root),
	}
	// add the hugetlb cgroup if error wasn't due to missing hugetlb
	// cgroup support on the host
	if err == nil {
		s = append(s, h)
	}
	return s, nil
}

func newMock(tb testing.TB, cgroupName string) (*mockCgroup, error) {
	root := tb.TempDir()
	subsystems, err := defaults(root)
	if err != nil {
		return nil, err
	}
	for _, s := range subsystems {
		if err := os.MkdirAll(filepath.Join(root, string(s.Name()), cgroupName), defaultDirPerm); err != nil {
			return nil, err
		}
	}
	// make cpuset root files
	for _, v := range []struct {
		name  string
		value []byte
	}{
		{
			name:  "cpuset.cpus",
			value: []byte("0-3"),
		},
		{
			name:  "cpuset.mems",
			value: []byte("0-3"),
		},
	} {
		if err := os.WriteFile(filepath.Join(root, "cpuset", cgroupName, v.name), v.value, defaultFilePerm); err != nil {
			return nil, err
		}
	}
	return &mockCgroup{
		root:       root,
		subsystems: subsystems,
	}, nil
}

func TestReadStat(t *testing.T) {
	RegisterFailHandler(Fail)
	g := NewWithT(t)

	if runtime.GOOS == "linux" {
		t.Run("Test Read Cgroup Stat", func(t *testing.T) {
			cgroupName := "test"
			mockGroup, err := newMock(t, cgroupName)
			g.Expect(err).To(BeNil())
			defer func() {
				if err = mockGroup.delete(); err != nil {
					t.Errorf("failed delete: %v", err)
				}
			}()
			control, err := cgroup1.New(cgroup1.StaticPath(cgroupName), &specs.LinuxResources{}, cgroup1.WithHiearchy(mockGroup.hierarchy))
			g.Expect(err).To(BeNil())
			s, err := control.Stat(cgroups.IgnoreNotExist)
			g.Expect(err).To(BeNil())
			g.Expect(s).NotTo(BeNil())
		})
	}
}
