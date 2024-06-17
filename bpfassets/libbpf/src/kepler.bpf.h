// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#ifndef NUM_CPUS
# define NUM_CPUS 128
#endif

#ifndef MAP_SIZE
# define MAP_SIZE 32768
#endif

#define TASK_RUNNING 0

#include <vmlinux_small.h>

#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>

/**
 * commit 2f064a59a1 ("sched: Change task_struct::state") changes
 * the name of task_struct::state to task_struct::__state
 * see:
 *     https://github.com/torvalds/linux/commit/2f064a59a1
 */
struct task_struct___o {
	volatile long int state;
} __attribute__((preserve_access_index));

struct task_struct___x {
	unsigned int __state;
} __attribute__((preserve_access_index));

static __always_inline __s64 get_task_state(void *task)
{
	struct task_struct___x *t = task;

	if (bpf_core_field_exists(t->__state))
		return BPF_CORE_READ(t, __state);
	return BPF_CORE_READ((struct task_struct___o *)task, state);
}

typedef struct process_metrics_t {
	u64 cgroup_id;
	u64 pid; // pid is the kernel space view of the thread id
	u64 process_run_time;
	u64 cpu_cycles;
	u64 cpu_instr;
	u64 cache_miss;
	u64 page_cache_hit;
	u16 vec_nr[10];
	char comm[16];
} process_metrics_t;

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