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
	"math"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
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
	re = regexp.MustCompile(`\d+(\.\d+)?\s?[G|M|g|m][H|h]z`)
	name = re.ReplaceAllString(name, "")

	return strings.TrimSpace(name)
}

func formatProcessor(processor string) string {
	name := rename(processor)
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ToLower(name)
	return strings.ReplaceAll(name, "_v", "v")
}

func formatVendor(vendor string) string {
	parts := strings.Fields(vendor)
	if len(parts) > 0 {
		return strings.ReplaceAll(parts[0], "-", "_")
	}
	return ""
}

func roundToNearestHundred(value float64) int {
	return int(math.Round(value/100) * 100)
}

func generateSpec() *MachineSpec {
	spec := &MachineSpec{}
	spec.Cores = -1
	spec.Memory = -1
	spec.Frequency = -1
	spec.Chips = 1
	spec.ThreadsPerCore = 1

	cpus, err := cpu.Info()
	if err == nil {
		if len(cpus) > 0 {
			spec.Processor = formatProcessor(cpus[0].ModelName)
			spec.Vendor = formatVendor(cpus[0].VendorID)
			spec.Frequency = roundToNearestHundred(cpus[0].Mhz) // rounded frequency
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
