// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// QEMU/KVM patterns - matches both qemu-system-* and qemu-kvm variants
	qemuPattern = regexp.MustCompile(`(bin/qemu-system-\w+|libexec/qemu-kvm)`)

	// TODO: add patterns for virtual box,  VMware, Xen

	// VM process name patterns
	vmProcessPatterns = map[*regexp.Regexp]Hypervisor{
		qemuPattern: KVMHypervisor,
	}
)

// vmInfoFromProc detects if a process is a VM process and extracts VM info
func vmInfoFromProc(proc procInfo) (*VirtualMachine, error) {
	// Check command line for VM processes
	cmdline, err := proc.CmdLine()
	if err != nil {
		return nil, fmt.Errorf("failed to get process cmdline: %w", err)
	}

	if len(cmdline) == 0 {
		return nil, nil
	}

	hypervisor, vmID := vmInfoFromCmdLine(cmdline)
	if hypervisor == UnknownHypervisor {
		return nil, nil
	}

	vm := &VirtualMachine{
		ID:         vmID,
		Hypervisor: hypervisor,
	}

	// Try to get VM name from command line arguments
	vm.Name = vmNameFromCmdLine(cmdline, hypervisor)

	if vm.Name == "" {
		vm.Name = fmt.Sprintf("%s-%s", hypervisor, vmID[:8])
	}

	return vm, nil
}

// vmInfoFromCmdLine extracts VM information from command line
func vmInfoFromCmdLine(cmdline []string) (Hypervisor, string) {
	if len(cmdline) == 0 {
		return UnknownHypervisor, ""
	}
	exe := filepath.Base(cmdline[0])
	fullCmd := strings.Join(cmdline, " ")

	for pattern, hypervisor := range vmProcessPatterns {
		if pattern.MatchString(exe) || pattern.MatchString(fullCmd) {
			vmID := extractVMID(cmdline, hypervisor)

			// If VM ID is still empty, make one up from the command line parameter hash
			// TODO: validate if this this is a good idea
			if vmID == "" {
				vmID = generateVMID(fullCmd)
			}
			return hypervisor, vmID
		}
	}

	return UnknownHypervisor, ""
}

// extractVMID extracts VM ID from command line arguments based on hypervisor
func extractVMID(cmdline []string, hypervisor Hypervisor) string {
	switch hypervisor {
	case KVMHypervisor:
		return extractQemuMachineID(cmdline)
	default:
		return ""
	}
}

// extractQemuMachineID extracts VM ID from QEMU/KVM command line arguments, if present
// otherwise returns the VM name
func extractQemuMachineID(cmdline []string) string {
	for i, arg := range cmdline {
		if arg == "-uuid" && i+1 < len(cmdline) {
			return cmdline[i+1]
		}
	}
	return qemuVMNameFromCmdLine(cmdline)
}

// generateVMID generates a VM ID when one can't be extracted
func generateVMID(fullCmd string) string {
	hash := fmt.Sprintf("%x", []byte(fullCmd))
	if len(hash) > 16 {
		return hash[:16]
	}
	return hash
}

// vmNameFromCmdLine extracts VM name from command line arguments
func vmNameFromCmdLine(cmdline []string, hypervisor Hypervisor) string {
	switch hypervisor {
	case KVMHypervisor:
		return qemuVMNameFromCmdLine(cmdline)
	default:
		return ""
	}
}

// qemuVMNameFromCmdLine extracts VM name from QEMU command line
func qemuVMNameFromCmdLine(cmdline []string) string {
	for i, arg := range cmdline {
		if arg == "-name" && i+1 < len(cmdline) {
			value := cmdline[i+1]
			// QEMU -name can have format "guest=name,debug-threads=on"
			if strings.Contains(value, "guest=") {
				parts := strings.Split(value, ",")

				for _, part := range parts {
					if strings.HasPrefix(part, "guest=") {
						return strings.TrimPrefix(part, "guest=")
					}
				}
			}
			return value
		}

		if strings.HasPrefix(arg, "-name=") {
			return strings.TrimPrefix(arg, "-name=")
		}
	}
	return ""
}
