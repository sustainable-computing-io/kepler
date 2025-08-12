// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMInfoFromCmdLine(t *testing.T) {
	type expect struct {
		hypervisor Hypervisor
		vmID       string
	}

	tests := []struct {
		name     string
		cmdline  []string
		expected expect
	}{{
		name: "QEMU with UUID",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "guest=test-vm,debug-threads=on",
			"-uuid", "df12672f-fedb-4f6f-9d51-0166868835fb",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "df12672f-fedb-4f6f-9d51-0166868835fb",
		},
	}, {
		name: "QEMU without UUID (uses guest name)",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "guest=test-vm,debug-threads=on",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "test-vm",
		},
	}, {
		name: "QEMU with simple name format",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "simple-vm",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "simple-vm",
		},
	}, {
		name: "QEMU with name= format",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name=test-vm",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "test-vm",
		},
	}, {
		name: "QEMU ARM64",
		cmdline: []string{
			"/usr/bin/qemu-system-aarch64",
			"-name", "guest=arm-vm",
			"-uuid", "12345678-1234-5678-9abc-123456789abc",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "12345678-1234-5678-9abc-123456789abc",
		},
	}, {
		name: "QEMU-KVM OpenStack (CentOS/RHEL) - realistic command from issue #2276",
		cmdline: []string{
			"/usr/libexec/qemu-kvm",
			"-name", "guest=instance-0000008b,debug-threads=on",
			"-S",
			"-object", `{"qom-type":"secret","id":"masterKey0","format":"raw","file":"/var/lib/libvirt/qemu/domain-25-instance-0000008b/master-key.aes"}`,
			"-machine", "pc-q35-rhel9.4.0,usb=off,dump-guest-core=off,hpet=off,acpi=on",
			"-accel", "kvm",
			"-cpu", "Broadwell-IBRS",
			"-uuid", "df12672f-fedb-4f6f-9d51-0166868835fb",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "df12672f-fedb-4f6f-9d51-0166868835fb",
		},
	}, {
		name: "QEMU-KVM OpenStack without UUID (CentOS/RHEL) - uses guest name",
		cmdline: []string{
			"/usr/libexec/qemu-kvm",
			"-name", "guest=instance-0000008b,debug-threads=on",
			"-S",
			"-object", `{"qom-type":"secret","id":"masterKey0","format":"raw","file":"/var/lib/libvirt/qemu/domain-25-instance-0000008b/master-key.aes"}`,
			"-machine", "pc-q35-rhel9.4.0,usb=off,dump-guest-core=off,hpet=off,acpi=on",
			"-accel", "kvm",
			"-cpu", "Broadwell-IBRS",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "instance-0000008b",
		},
	}, {
		name: "Not a VM process",
		cmdline: []string{
			"/usr/bin/firefox",
			"--profile", "/home/user/.mozilla/firefox",
		},
		expected: expect{
			hypervisor: UnknownHypervisor,
			vmID:       "",
		},
	}, {
		name:    "Empty cmdline",
		cmdline: []string{},
		expected: expect{
			hypervisor: UnknownHypervisor,
			vmID:       "",
		},
	}, {
		name: "QEMU without any name info (generates hash-based ID)",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-machine", "pc",
			"-m", "1024",
		},
		expected: expect{
			hypervisor: KVMHypervisor,
			vmID:       "2f7573722f62696e", // First 16 chars of hex hash
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hypervisor, vmID := vmInfoFromCmdLine(tc.cmdline)
			assert.Equal(t, tc.expected.hypervisor, hypervisor)
			assert.Equal(t, tc.expected.vmID, vmID)
		})
	}
}

func TestQemuVMNameFromCmdLine(t *testing.T) {
	tests := []struct {
		name     string
		cmdline  []string
		expected string
	}{{
		name: "Guest name from complex format",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "guest=kepler-stuff_default,debug-threads=on",
		},
		expected: "kepler-stuff_default",
	}, {
		name: "Simple name format",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "simple-vm",
		},
		expected: "simple-vm",
	}, {
		name: "Name with equals format",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name=test-vm",
		},
		expected: "test-vm",
	}, {
		name: "No name argument",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-machine", "pc",
		},
		expected: "",
	}, {
		name: "Name argument without value",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name",
		},
		expected: "",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := qemuVMNameFromCmdLine(tc.cmdline)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractQemuMachineID(t *testing.T) {
	tests := []struct {
		name     string
		cmdline  []string
		expected string
	}{{
		name: "UUID present",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-uuid", "df12672f-fedb-4f6f-9d51-0166868835fb",
			"-name", "guest=test-vm",
		},
		expected: "df12672f-fedb-4f6f-9d51-0166868835fb",
	}, {
		name: "No UUID, fallback to VM name",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "guest=test-vm,debug-threads=on",
		},
		expected: "test-vm",
	}, {
		name: "No UUID, no name",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-machine", "pc",
		},
		expected: "",
	}, {
		name: "UUID without value",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-uuid",
		},
		expected: "",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractQemuMachineID(tc.cmdline)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateVMID(t *testing.T) {
	tests := []struct {
		name     string
		fullCmd  string
		expected string
	}{{
		name:     "Short command",
		fullCmd:  "/usr/bin/qemu",
		expected: "2f7573722f62696e",
	}, {
		name:     "Long command gets truncated",
		fullCmd:  "/usr/bin/qemu-system-x86_64 -name guest=test-vm -uuid 12345678-1234-5678-9abc-123456789abc",
		expected: "2f7573722f62696e", // First 16 chars
	}, {
		name:     "Empty command",
		fullCmd:  "",
		expected: "",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := generateVMID(tc.fullCmd)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVMInfoFromProc(t *testing.T) {
	type expect struct {
		vm    *VirtualMachine
		error bool
	}

	tests := []struct {
		name         string
		cmdline      []string
		cmdlineError error
		expected     expect
	}{{
		name: "QEMU VM with complete info",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "guest=kepler-stuff_default,debug-threads=on",
			"-uuid", "df12672f-fedb-4f6f-9d51-0166868835fb",
		},
		expected: expect{
			vm: &VirtualMachine{
				ID:         "df12672f-fedb-4f6f-9d51-0166868835fb",
				Name:       "kepler-stuff_default",
				Hypervisor: KVMHypervisor,
			},
			error: false,
		},
	}, {
		name: "QEMU VM without UUID",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-name", "guest=test-vm",
		},
		expected: expect{
			vm: &VirtualMachine{
				ID:         "test-vm",
				Name:       "test-vm",
				Hypervisor: KVMHypervisor,
			},
			error: false,
		},
	}, {
		name: "QEMU VM without name (generates name)",
		cmdline: []string{
			"/usr/bin/qemu-system-x86_64",
			"-uuid", "df12672f-fedb-4f6f-9d51-0166868835fb",
		},
		expected: expect{
			vm: &VirtualMachine{
				ID:         "df12672f-fedb-4f6f-9d51-0166868835fb",
				Name:       "kvm-df12672f",
				Hypervisor: KVMHypervisor,
			},
			error: false,
		},
	}, {
		name: "Not a VM process",
		cmdline: []string{
			"/usr/bin/firefox",
			"--profile", "/home/user/.mozilla/firefox",
		},
		expected: expect{nil, false},
	}, {
		name:         "Error reading cmdline",
		cmdline:      nil,
		cmdlineError: assert.AnError,
		expected:     expect{nil, true},
	}, {
		name:     "Empty cmdline",
		cmdline:  []string{},
		expected: expect{nil, false},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockProc := &MockProcInfo{}
			mockProc.On("CmdLine").Return(tc.cmdline, tc.cmdlineError)

			vm, err := vmInfoFromProc(mockProc)

			if tc.expected.error {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tc.expected.vm == nil {
				assert.Nil(t, vm)
				return
			}

			require.NotNil(t, vm)
			assert.Equal(t, tc.expected.vm.ID, vm.ID)
			assert.Equal(t, tc.expected.vm.Name, vm.Name)
			assert.Equal(t, tc.expected.vm.Hypervisor, vm.Hypervisor)

			mockProc.AssertExpectations(t)
		})
	}
}

func TestVMClone(t *testing.T) {
	t.Run("Full VM clone", func(t *testing.T) {
		original := &VirtualMachine{
			ID:           "df12672f-fedb-4f6f-9d51-0166868835fb",
			Name:         "test-vm",
			Hypervisor:   KVMHypervisor,
			CPUTimeDelta: 123.45,
		}

		clone := original.Clone()

		// Check that the clone has the same values
		assert.Equal(t, original.ID, clone.ID)
		assert.Equal(t, original.Name, clone.Name)
		assert.Equal(t, original.Hypervisor, clone.Hypervisor)
		assert.Equal(t, float64(0), clone.CPUTimeDelta) // CPUTime shouldn't be cloned
	})

	t.Run("Clone nil VM", func(t *testing.T) {
		var nilVM *VirtualMachine
		nilClone := nilVM.Clone()
		assert.Nil(t, nilClone, "Cloning nil VM should return nil")
	})
}

func TestExtractVMID(t *testing.T) {
	tests := []struct {
		name       string
		cmdline    []string
		hypervisor Hypervisor
		expected   string
	}{{
		name:       "no UUID or name returns empty",
		cmdline:    []string{"/usr/bin/qemu-system-x86_64", "-m", "1024"},
		hypervisor: KVMHypervisor,
		expected:   "",
	}, {
		name:       "unknown hypervisor returns empty",
		cmdline:    []string{"/usr/bin/qemu-system-x86_64", "-m", "1024"},
		hypervisor: UnknownHypervisor,
		expected:   "",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id := extractVMID(tc.cmdline, tc.hypervisor)
			assert.Equal(t, tc.expected, id)
		})
	}
}

func TestVMNameFromCmdLine(t *testing.T) {
	tests := []struct {
		name       string
		cmdline    []string
		hypervisor Hypervisor
		expected   string
	}{{
		name:       "empty cmdline returns empty",
		cmdline:    []string{},
		hypervisor: KVMHypervisor,
		expected:   "",
	}, {
		name:       "non-KVM hypervisor returns empty",
		cmdline:    []string{"/usr/bin/qemu-system-x86_64", "-name", "test"},
		hypervisor: UnknownHypervisor,
		expected:   "",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name := vmNameFromCmdLine(tc.cmdline, tc.hypervisor)
			assert.Equal(t, tc.expected, name)
		})
	}
}
