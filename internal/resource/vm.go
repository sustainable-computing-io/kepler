// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	// QEMU/KVM patterns - matches both qemu-system-* and qemu-kvm variants
	qemuPattern = regexp.MustCompile(`(bin/qemu-system-\w+|libexec/qemu-kvm|bin/kvm)`)

	// VirtualBox patterns - matches the per-VM headless, SDL and GUI processes
	virtualBoxPattern = regexp.MustCompile(`virtualbox/(VBoxHeadless|VBoxSDL|VirtualBoxVM)`)

	// VMware patterns - vmware-vmx runs as one process per powered-on VM
	vmwarePattern = regexp.MustCompile(`bin/vmware-vmx`)

	// VM process name patterns, checked in order so that classification stays
	// deterministic even if a command line matches more than one pattern
	vmProcessPatterns = []struct {
		pattern    *regexp.Regexp
		hypervisor Hypervisor
	}{
		{qemuPattern, KVMHypervisor},
		{virtualBoxPattern, VirtualBoxHypervisor},
		{vmwarePattern, VMwareHypervisor},
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
		shortID := vmID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		vm.Name = fmt.Sprintf("%s-%s", hypervisor, shortID)
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

	for _, p := range vmProcessPatterns {
		if p.pattern.MatchString(exe) || p.pattern.MatchString(fullCmd) {
			hypervisor := p.hypervisor

			// Xen launches its device model through QEMU; the -xen-domid flag
			// distinguishes it from a plain QEMU/KVM guest
			if hypervisor == KVMHypervisor && hasXenDomID(cmdline) {
				hypervisor = XenHypervisor
			}

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

// hasXenDomID reports whether the command line carries the -xen-domid flag
// that Xen passes to its QEMU based device model
func hasXenDomID(cmdline []string) bool {
	return slices.Contains(cmdline, "-xen-domid")
}

// extractVMID extracts VM ID from command line arguments based on hypervisor
func extractVMID(cmdline []string, hypervisor Hypervisor) string {
	switch hypervisor {
	case KVMHypervisor:
		return extractQemuMachineID(cmdline)
	case VirtualBoxHypervisor:
		return extractVBoxMachineID(cmdline)
	case VMwareHypervisor:
		return vmwareVMNameFromCmdLine(cmdline)
	case XenHypervisor:
		return extractXenDomID(cmdline)
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

// extractVBoxMachineID extracts VM ID from VirtualBox command line arguments,
// if present otherwise returns the VM name
func extractVBoxMachineID(cmdline []string) string {
	for i, arg := range cmdline {
		if strings.TrimLeft(arg, "-") == "startvm" && i+1 < len(cmdline) {
			return cmdline[i+1]
		}
	}
	return vboxVMNameFromCmdLine(cmdline)
}

// extractXenDomID extracts the Xen domain ID from the device model command
// line, if present otherwise returns the VM name
func extractXenDomID(cmdline []string) string {
	for i, arg := range cmdline {
		if arg == "-xen-domid" && i+1 < len(cmdline) {
			return cmdline[i+1]
		}
	}
	// Xen device model uses QEMU style -name arguments
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
	case KVMHypervisor, XenHypervisor:
		return qemuVMNameFromCmdLine(cmdline)
	case VirtualBoxHypervisor:
		return vboxVMNameFromCmdLine(cmdline)
	case VMwareHypervisor:
		return vmwareVMNameFromCmdLine(cmdline)
	default:
		return ""
	}
}

// vboxVMNameFromCmdLine extracts VM name from VirtualBox command line; the
// per-VM processes carry the VM name in the --comment argument
func vboxVMNameFromCmdLine(cmdline []string) string {
	for i, arg := range cmdline {
		if strings.TrimLeft(arg, "-") == "comment" && i+1 < len(cmdline) {
			return cmdline[i+1]
		}
	}
	return ""
}

// vmwareVMNameFromCmdLine extracts VM name from the .vmx config file path
// that vmware-vmx receives as an argument
func vmwareVMNameFromCmdLine(cmdline []string) string {
	for _, arg := range cmdline {
		if strings.HasSuffix(arg, ".vmx") {
			return strings.TrimSuffix(filepath.Base(arg), ".vmx")
		}
	}
	return ""
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
