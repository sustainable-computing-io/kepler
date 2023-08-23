// Code generated for package bpfassets by go-bindata DO NOT EDIT. (@generated)
// sources:
// bpfassets/perf_event/perf_event.c
package bpfassets

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _bpfassetsPerf_eventPerf_eventC = []byte(`/*
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

#include <linux/sched.h>
// #include <linux/bpf.h>
// #include <linux/bpf_perf_event.h>

#ifndef NUM_CPUS
#define NUM_CPUS 128
#endif

#ifndef CPU_REF_FREQ
#define CPU_REF_FREQ 2500
#endif

#ifndef HZ
#define HZ 1000
#endif

#ifndef MAP_SIZE
#define MAP_SIZE 10240
#endif

typedef struct process_metrics_t
{
    u64 cgroup_id;
    u64 pid;
    u64 process_run_time;
    u64 cpu_cycles;
    u64 cpu_instr;
    u64 cache_miss;
    u16 vec_nr[10]; // irq counter, 10 is the max number of irq vectors
    char comm[16];
} process_metrics_t;

typedef struct pid_time_t
{
    pid_t pid;
} pid_time_t;

// processes and pid time
BPF_HASH(processes, pid_t, process_metrics_t, MAP_SIZE);
BPF_HASH(pid_time, pid_t);

// perf counters
BPF_PERF_ARRAY(cpu_cycles_hc_reader, NUM_CPUS);
BPF_ARRAY(cpu_cycles, u64, NUM_CPUS);

BPF_PERF_ARRAY(cpu_ref_cycles_hc_reader, NUM_CPUS);
BPF_ARRAY(cpu_ref_cycles, u64, NUM_CPUS);

BPF_PERF_ARRAY(cpu_instr_hc_reader, NUM_CPUS);
BPF_ARRAY(cpu_instr, u64, NUM_CPUS);

BPF_PERF_ARRAY(cache_miss_hc_reader, NUM_CPUS);
BPF_ARRAY(cache_miss, u64, NUM_CPUS);

// cpu freq counters
BPF_ARRAY(cpu_freq_array, u32, NUM_CPUS);

static inline u64 get_on_cpu_time(pid_t cur_pid, u32 prev_pid, u32 cpu_id, u64 cur_ts)
{
    u64 cpu_time = 0;

    // get pid time
    u64 *prev_ts = pid_time.lookup(&prev_pid);
    if (prev_ts != 0)
    {
        // Probably a clock issue where the recorded on-CPU event had a
        // timestamp later than the recorded off-CPU event, or vice versa.
        // But do not return, since the hardware counters can be collected.
        if (cur_ts > *prev_ts)
        {
            cpu_time = (cur_ts - *prev_ts) / 1000000; /*milisecond*/
            pid_time.delete(&prev_pid);
        }
    }
    pid_time.update(&cur_pid, &cur_ts);

    return cpu_time;
}

static inline u64 normalize(u64 *counter, u64 *enabled, u64 *running)
{
    if (*running > 0)
        return *counter * *enabled / *running;
    return *counter;
}

static inline u64 calc_delta(u64 *prev_val, u64 *val)
{
    u64 delta = 0;
    if (prev_val)
    {
        if (*val > *prev_val)
            delta = *val - *prev_val;
    }
    return delta;
}

// altough the "get_on_cpu_counters" has some code duplications, it is inline code and the compiles will improve this
static inline u64 get_on_cpu_cycles(u32 *cpu_id)
{
    u64 delta = 0;
    struct bpf_perf_event_value c = {};
    int error = cpu_cycles_hc_reader.perf_counter_value(CUR_CPU_IDENTIFIER, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = cpu_cycles.lookup(cpu_id);
        delta = calc_delta(prev_val, &val);
        cpu_cycles.update(cpu_id, &val);
    }
    return delta;
}

static inline u64 get_on_cpu_ref_cycles(u32 *cpu_id)
{
    u64 delta = 0;
    struct bpf_perf_event_value c = {};
    int error = cpu_ref_cycles_hc_reader.perf_counter_value(CUR_CPU_IDENTIFIER, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = cpu_ref_cycles.lookup(cpu_id);
        delta = calc_delta(prev_val, &val);
        cpu_ref_cycles.update(cpu_id, &val);
    }
    return delta;
}

static inline u64 get_on_cpu_instr(u32 *cpu_id)
{
    u64 delta = 0;
    struct bpf_perf_event_value c = {};
    int error = cpu_instr_hc_reader.perf_counter_value(CUR_CPU_IDENTIFIER, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = cpu_instr.lookup(cpu_id);
        delta = calc_delta(prev_val, &val);
        cpu_instr.update(cpu_id, &val);
    }
    return delta;
}

static inline u64 get_on_cpu_cache_miss(u32 *cpu_id)
{
    u64 delta = 0;
    struct bpf_perf_event_value c = {};
    int error = cache_miss_hc_reader.perf_counter_value(CUR_CPU_IDENTIFIER, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = cache_miss.lookup(cpu_id);
        delta = calc_delta(prev_val, &val);
        cache_miss.update(cpu_id, &val);
    }
    return delta;
}

// calculate the average cpu freq
static inline u64 get_on_cpu_avg_freq(u32 *cpu_id, u64 on_cpu_cycles_delta, u64 on_cpu_ref_cycles_delta)
{
    u32 avg_freq = 0;
    cpu_freq_array.lookup_or_try_init(cpu_id, &avg_freq);
    if (avg_freq == 0)
    {
        avg_freq = ((on_cpu_cycles_delta * CPU_REF_FREQ) / on_cpu_ref_cycles_delta) * HZ;
    }
    else
    {
        avg_freq += ((on_cpu_cycles_delta * CPU_REF_FREQ) / on_cpu_ref_cycles_delta) * HZ;
        avg_freq /= 2;
    }
    return avg_freq;
}

// int kprobe__finish_task_switch(switch_args *ctx)
int kprobe__finish_task_switch(struct pt_regs *ctx, struct task_struct *prev)
{
    pid_t cur_pid = bpf_get_current_pid_tgid();
#ifdef SET_GROUP_ID
    u64 cgroup_id = bpf_get_current_cgroup_id();
#else
    u64 cgroup_id = 0;
#endif

    u64 cur_ts = bpf_ktime_get_ns();
    u32 cpu_id = bpf_get_smp_processor_id();
    pid_t prev_pid = prev->pid;
    u64 on_cpu_time_delta = get_on_cpu_time(cur_pid, prev_pid, cpu_id, cur_ts);
    u64 on_cpu_cycles_delta = get_on_cpu_cycles(&cpu_id);
    u64 on_cpu_ref_cycles_delta = get_on_cpu_ref_cycles(&cpu_id);
    u64 on_cpu_instr_delta = get_on_cpu_instr(&cpu_id);
    u64 on_cpu_cache_miss_delta = get_on_cpu_cache_miss(&cpu_id);
    u64 on_cpu_avg_freq = get_on_cpu_avg_freq(&cpu_id, on_cpu_cycles_delta, on_cpu_ref_cycles_delta);

    // store process metrics
    struct process_metrics_t *process_metrics;
    process_metrics = processes.lookup(&prev_pid);
    if (process_metrics != 0)
    {
        // update process time
        process_metrics->process_run_time += on_cpu_time_delta;

        process_metrics->cpu_cycles += on_cpu_cycles_delta;
        process_metrics->cpu_instr += on_cpu_instr_delta;
        process_metrics->cache_miss += on_cpu_cache_miss_delta;
    }

    process_metrics = processes.lookup(&cur_pid);
    if (process_metrics == 0)
    {
        process_metrics_t new_process = {};
        new_process.pid = cur_pid;
        new_process.cgroup_id = cgroup_id;
        bpf_get_current_comm(&new_process.comm, sizeof(new_process.comm));
        processes.update(&cur_pid, &new_process);
    }

    return 0;
}

// per https://www.kernel.org/doc/html/latest/core-api/tracepoint.html#c.trace_softirq_entry
TRACEPOINT_PROBE(irq, softirq_entry)
{
    pid_t cur_pid = bpf_get_current_pid_tgid();
    struct process_metrics_t *process_metrics;
    process_metrics = processes.lookup(&cur_pid);
    if (process_metrics != 0)
    {
        if (args->vec < 10) {
            process_metrics->vec_nr[args->vec] ++;
        }
    }
    return 0;
}
`)

func bpfassetsPerf_eventPerf_eventCBytes() ([]byte, error) {
	return _bpfassetsPerf_eventPerf_eventC, nil
}

func bpfassetsPerf_eventPerf_eventC() (*asset, error) {
	bytes, err := bpfassetsPerf_eventPerf_eventCBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "bpfassets/perf_event/perf_event.c", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"bpfassets/perf_event/perf_event.c": bpfassetsPerf_eventPerf_eventC,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"bpfassets": {nil, map[string]*bintree{
		"perf_event": {nil, map[string]*bintree{
			"perf_event.c": {bpfassetsPerf_eventPerf_eventC, map[string]*bintree{}},
		}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
