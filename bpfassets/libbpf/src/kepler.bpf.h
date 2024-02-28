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


/* In Linux 5.4 asm_inline was introduced, but it's not supported by clang.
 * Redefine it to just asm to enable successful compilation.
 * see https://github.com/iovisor/bcc/commit/2d1497cde1cc9835f759a707b42dea83bee378b8 for more details
 */
#if defined(__TARGET_ARCH_x86)
#include "vmlinux-x86.h"
#endif
#if defined(__TARGET_ARCH_arm64)
#include "vmlinux-arm64.h"
#endif
#if defined(__TARGET_ARCH_s390)
#include "vmlinux-s390.h"
#endif
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>

#ifdef asm_inline
#undef asm_inline
#define asm_inline asm
#endif

typedef __u64 u64;
typedef __u32 u32;
typedef __u16 u16;

#ifndef NUM_CPUS
#define NUM_CPUS 128
#endif

#ifndef TASK_COMM_LEN
#define TASK_COMM_LEN 16
#endif

// irq counter, 10 is the max number of irq vectors
#ifndef IRQ_MAX_LEN 
#define IRQ_MAX_LEN 10
#endif

#ifndef CPU_REF_FREQ
#define CPU_REF_FREQ 2500
#endif

#ifndef HZ
#define HZ 1000
#endif

#ifndef MAP_SIZE
#define MAP_SIZE 32768
#endif

// array size is to be reset in userspace
#define BPF_ARRARY_MAP(_name, _type, _key_type, _value_type) \
    struct {                                                 \
        __uint(type, _type);                                 \
        __type(key, _key_type);                              \
        __type(value, _value_type);                          \
        __uint(max_entries, NUM_CPUS);                       \
    } _name SEC(".maps");

#define BPF_HASH(_name, _key_type, _value_type) \
    struct {                                    \
        __uint(type, BPF_MAP_TYPE_HASH);        \
        __type(key, _key_type);                 \
        __type(value, _value_type);             \
        __uint(max_entries, MAP_SIZE);          \
    } _name SEC(".maps");

#define BPF_ARRAY(_name, _leaf_type) \
    BPF_ARRARY_MAP(_name, BPF_MAP_TYPE_ARRAY, u32, _leaf_type);

#define BPF_PERF_ARRAY(_name) \
    BPF_ARRARY_MAP(_name, BPF_MAP_TYPE_PERF_EVENT_ARRAY, int, u32)

static __always_inline void *
bpf_map_lookup_or_try_init(void *map, const void *key, const void *init)
{
	void *val;
	int err;

	val = bpf_map_lookup_elem(map, key);
	if (val)
		return val;

	err = bpf_map_update_elem(map, key, init, BPF_NOEXIST);
	if (err && err != -17)
		return 0;

	return bpf_map_lookup_elem(map, key);
}

struct sched_switch_args {
    unsigned long long pad;
    char prev_comm[TASK_COMM_LEN];
    int prev_pid;
    int prev_prio;
    long long prev_state;
    char next_comm[TASK_COMM_LEN];
    int next_pid;
    int next_prio;
};

/*
struct trace_event_raw_softirq {
    unsigned long long pad;
    unsigned int vec;
};
*/

typedef struct process_metrics_t
{
    u64 cgroup_id;
    u64 pid; // pid is the kernel space view of the thread id
    u64 tgid; // tgid is the user space view of the pid
    u64 process_run_time;
    u64 task_clock_time;
    u64 cpu_cycles;
    u64 cpu_instr;
    u64 cache_miss;
    u64 page_cache_hit;
    u16 vec_nr[IRQ_MAX_LEN]; 
    char comm[TASK_COMM_LEN];
} process_metrics_t;

typedef struct pid_time_t
{
    u32 pid;
} pid_time_t;

