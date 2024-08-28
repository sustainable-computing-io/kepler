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

package config

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"k8s.io/klog/v2"
)

func rename(name string) string {
	replacements := []struct {
		old, new string
	}{
		{"(R)", ""},
		{"(r)", ""},
		{"CPU", ""},
		{"Processor", ""},
		{"processor", ""},
		{"@", ""},
		{"®", ""},
		{",", ""},
	}

	for _, r := range replacements {
		name = strings.ReplaceAll(name, r.old, r.new)
	}

	// Remove "n-Bit Multi-Core" pattern
	re := regexp.MustCompile(`\d+-Bit Multi-Core`)
	name = re.ReplaceAllString(name, "")

	// Remove anything in parentheses or square brackets
	re = regexp.MustCompile(`[\(\[].*?[\)\]]`)
	name = re.ReplaceAllString(name, "")

	// Remove frequency pattern
	re = regexp.MustCompile(`\d+(\.\d+)?\s?[GMgm][Hh]z`)
	name = re.ReplaceAllString(name, "")

	return strings.TrimSpace(name)
}

func formatProcessor(processor string) string {
	name := rename(processor)
	name = strings.ToLower(name)
	// replace spaces and hyphens with an underscore
	name = strings.ReplaceAll(name, "-", "_")
	fields := strings.Fields(name)
	name = strings.Join(fields, "_")
	return strings.ReplaceAll(name, "_v", "v")
}

func formatVendor(vendor string) string {
	vendor = strings.ToLower(vendor)
	parts := strings.Fields(vendor)
	if len(parts) > 0 {
		vendor = parts[0]
	}
	vendor = strings.ReplaceAll(vendor, "-", "_")
	vendor = strings.ReplaceAll(vendor, ",", "")
	vendor = strings.ReplaceAll(vendor, "'", "")
	return vendor
}

func roundToNearestHundred(value float64) int {
	return int(math.Round(value/100) * 100)
}

func GenerateSpec() *MachineSpec {
	spec := &MachineSpec{}

	cpus, err := cpu.Info()

	if err == nil {
		if len(cpus) > 0 {
			spec.Processor = formatProcessor(cpus[0].ModelName)
			spec.Vendor = formatVendor(cpus[0].VendorID)
			spec.Frequency = roundToNearestHundred(cpus[0].Mhz) // rounded frequency
			physicalIDMap := make(map[string]bool)
			for index := range cpus {
				physicalIDMap[cpus[index].PhysicalID] = true
			}
			if physicalIDCount := len(physicalIDMap); physicalIDCount > 0 {
				spec.Chips = physicalIDCount
			}
		}
	}

	if cores, err := cpu.Counts(true); err == nil {
		spec.Cores = cores
	}
	// Threads per core calculation
	if coresPhysical, err := cpu.Counts(false); err == nil {
		spec.ThreadsPerCore = spec.Cores / coresPhysical
	}
	// Memory info
	if vmStat, err := mem.VirtualMemory(); err == nil {
		spec.Memory = int(vmStat.Total / (1024 * 1024 * 1024)) // in GB
	}
	return spec
}

// getDefaultMachineSpec reads spec from default spec path only if the path was mounted (VM case).
// otherwise, generate spec (BM case)
func getDefaultMachineSpec() *MachineSpec {
	if utils.IsFileExists(DefaultMachineSpecFilePath) {
		if spec, err := readMachineSpec(DefaultMachineSpecFilePath); err == nil {
			return spec
		} else {
			klog.Errorf("failed to read default spec from %s: %v", DefaultMachineSpecFilePath, err)
		}
	}
	return GenerateSpec()
}

func readMachineSpec(path string) (*MachineSpec, error) {
	var spec *MachineSpec
	specFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open machine spec: %w", err)
	}

	specBytes, err := io.ReadAll(specFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read machine spec: %w", err)
	}

	if err = json.Unmarshal(specBytes, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal machine spec: %v", err)
	}

	klog.V(3).Infof("Read machine spec from config: %v", spec)
	return spec, nil
}

// MachineSpec defines a machine spec to submit for power model selection
type MachineSpec struct {
	Vendor         string `json:"vendor"`
	Processor      string `json:"processor"`
	Cores          int    `json:"cores"`
	Chips          int    `json:"chips"`
	Memory         int    `json:"memory"`
	Frequency      int    `json:"frequency"`
	ThreadsPerCore int    `json:"threads_per_core"`
}
