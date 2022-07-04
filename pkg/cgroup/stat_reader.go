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

var MEM_USAGE_FILES = []string {
	"memory.usage_in_bytes", // hierarchy: system + kernel
	"memory.kmem.usage_in_bytes", // hierarchy: kernel
	"memory.kmem.tcp.usage_in_bytes", // hierarchy: tcp buff
	"memory.current", // toppath memory stat
}

var CPU_USAGE_FILES = []string {
	"cpuacct.usage", // hierarchy: system + kernel
	"cpuacct.usage_sys", // hierarchy: kernel
	"cpuacct.usage_user", // hierarchy: tcp buff
	"cpu.stat", // toppath cpu stat
}

var IO_USAGE_FILES = []string {
	"io.stat",
}

var  STANDARD_METRIC_NAME_MAPS = map[string][]CgroupFSReadMetric{
		"cgroupfs_memory_usage_bytes": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "memory.current", Converter: DefaultConverter},
			CgroupFSReadMetric{ Name: "memory.usage_in_bytes", Converter: DefaultConverter},
		},
		"cgroupfs_kernel_memory_usage_bytes": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "memory.kmem.usage_in_bytes", Converter: DefaultConverter },
		},
		"cgroupfs_tcp_memory_usage_bytes": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "memory.kmem.tcp.usage_in_bytes", Converter: DefaultConverter },
		},
		"cgroupfs_cpu_usage_us": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "cpuacct.usage", Converter: NanoToMicroConverter },
			CgroupFSReadMetric{ Name: "usage_usec", Converter: DefaultConverter },
		},
		"cgroupfs_system_cpu_usage_us": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "cpuacct.usage_sys", Converter: NanoToMicroConverter},
			CgroupFSReadMetric{ Name: "system_usec", Converter: DefaultConverter },
		},
		"cgroupfs_user_cpu_usage_us": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "cpuacct.usage_user", Converter: NanoToMicroConverter },
			CgroupFSReadMetric{ Name: "usage_usec", Converter: DefaultConverter },
		},
		"cgroupfs_ioread_bytes": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "rbytes", Converter: DefaultConverter },
		},
		"cgroupfs_iowrite_bytes": []CgroupFSReadMetric{
			CgroupFSReadMetric{ Name: "wbytes", Converter: DefaultConverter },
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
	for _, usageFile := range MEM_USAGE_FILES {
		value, err := ReadUInt64(s.Path, usageFile)
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
	for _, usageFile := range CPU_USAGE_FILES {
		switch usageFile {
		case "cpu.stat":
			kv, err := ReadKV(s.Path, usageFile)
			if err == nil {
				return kv
			}
		default:
			value, err := ReadUInt64(s.Path, usageFile)
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
	for _, usageFile := range IO_USAGE_FILES {
		switch usageFile {
		case "io.stat":
			kv, err := ReadLineKEqualToV(s.Path, usageFile)
			if err == nil {
				return kv
			}
		}
	}
	return values
}


type CgroupFSReadMetric struct {
	Name string
	Converter func(stats map[string]interface{}, key string) interface{}
}

func DefaultConverter(stats map[string]interface{}, key string) interface{} {
	return stats[key]
}

func NanoToMicroConverter(stats map[string]interface{}, key string) interface{} {
	return uint64(stats[key].(uint64)/1000)
}

func convertToStandard(stats map[string]interface{}) map[string]interface{} {
	values := make(map[string]interface{})
	for key, readMetrics := range STANDARD_METRIC_NAME_MAPS {
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