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

package cgroup

import (
	"path/filepath"
)

var MemUsageFiles = []string{
	"memory.usage_in_bytes",          // hierarchy: system + kernel
	"memory.kmem.usage_in_bytes",     // hierarchy: kernel
	"memory.kmem.tcp.usage_in_bytes", // hierarchy: tcp buff
	"memory.current",                 // toppath memory stat
}

var cpuUsageFiles = []string{
	"cpuacct.usage",      // hierarchy: system + kernel
	"cpuacct.usage_sys",  // hierarchy: kernel
	"cpuacct.usage_user", // hierarchy: tcp buff
	"cpu.stat",           // toppath cpu stat
}

var ioUsageFiles = []string{
	"io.stat",
}

var standardMetricName = map[string][]CgroupFSReadMetric{
	"cgroupfs_memory_usage_bytes": {
		{Name: "memory.current", Converter: DefaultConverter},
		{Name: "memory.usage_in_bytes", Converter: DefaultConverter},
	},
	"cgroupfs_kernel_memory_usage_bytes": {
		{Name: "memory.kmem.usage_in_bytes", Converter: DefaultConverter},
	},
	"cgroupfs_tcp_memory_usage_bytes": {
		{Name: "memory.kmem.tcp.usage_in_bytes", Converter: DefaultConverter},
	},
	"cgroupfs_cpu_usage_us": {
		{Name: "cpuacct.usage", Converter: NanoToMicroConverter},
		{Name: "usage_usec", Converter: DefaultConverter},
	},
	"cgroupfs_system_cpu_usage_us": {
		{Name: "cpuacct.usage_sys", Converter: NanoToMicroConverter},
		{Name: "system_usec", Converter: DefaultConverter},
	},
	"cgroupfs_user_cpu_usage_us": {
		{Name: "cpuacct.usage_user", Converter: NanoToMicroConverter},
		{Name: "user_usec", Converter: DefaultConverter},
	},
	"cgroupfs_ioread_bytes": {
		{Name: "rbytes", Converter: DefaultConverter},
	},
	"cgroupfs_iowrite_bytes": {
		{Name: "wbytes", Converter: DefaultConverter},
	},
}

type StatReader interface {
	Read() map[string]interface{}
}

type MemoryStatReader struct {
	Path string
}

func (s MemoryStatReader) Read() map[string]interface{} {
	values := make(map[string]interface{})
	for _, usageFile := range MemUsageFiles {
		fileName := filepath.Join(s.Path, usageFile)
		value, err := ReadUInt64(fileName)
		if err == nil {
			values[usageFile] = value
		}
	}
	return values
}

type CPUStatReader struct {
	Path string
}

func (s CPUStatReader) Read() map[string]interface{} {
	values := make(map[string]interface{})
	for _, usageFile := range cpuUsageFiles {
		switch usageFile {
		case "cpu.stat":
			fileName := filepath.Join(s.Path, usageFile)
			kv, err := ReadKV(fileName)
			if err == nil {
				return kv
			}
		default:
			fileName := filepath.Join(s.Path, usageFile)
			value, err := ReadUInt64(fileName)
			if err == nil {
				values[usageFile] = value
			}
		}
	}
	return values
}

type IOStatReader struct {
	Path string
}

func (s IOStatReader) Read() map[string]interface{} {
	values := make(map[string]interface{})
	for _, usageFile := range ioUsageFiles {
		if usageFile == "io.stat" {
			kv, err := ReadLineKEqualToV(s.Path, usageFile)
			if err == nil {
				return kv
			}
		}
	}
	return values
}

type CgroupFSReadMetric struct {
	Name      string
	Converter func(stats map[string]interface{}, key string) interface{}
}

func DefaultConverter(stats map[string]interface{}, key string) interface{} {
	return stats[key]
}

func NanoToMicroConverter(stats map[string]interface{}, key string) interface{} {
	return stats[key].(uint64) / 1000
}

func convertToStandard(stats map[string]interface{}) map[string]interface{} {
	values := make(map[string]interface{})
	for key, readMetrics := range standardMetricName {
		for _, readMetric := range readMetrics {
			if _, exists := stats[readMetric.Name]; exists {
				value := readMetric.Converter(stats, readMetric.Name)
				values[key] = value
				break
			}
		}
	}
	return values
}
