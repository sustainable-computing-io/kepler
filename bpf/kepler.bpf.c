// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#include "kepler.bpf.h"

SEC("tp_btf/sched_switch")
int kepler_sched_switch_trace(u64 *ctx)
{
	struct task_struct *prev_task, *next_task;

	prev_task = (struct task_struct *)ctx[1];
	next_task = (struct task_struct *)ctx[2];

	return do_kepler_sched_switch_trace(
		prev_task->pid, next_task->pid, prev_task->tgid, next_task->tgid);
}

SEC("tp_btf/softirq_entry")
int kepler_irq_trace(u64 *ctx)
{
	u32 curr_tgid;
	struct process_metrics_t *process_metrics;
	unsigned int vec;

	curr_tgid = bpf_get_current_pid_tgid() >> 32;
	vec = (unsigned int)ctx[0];
	process_metrics = bpf_map_lookup_elem(&processes, &curr_tgid);
	if (process_metrics != 0 && vec < 10)
		process_metrics->vec_nr[vec] += 1;
	return 0;
}

// count read page cache
SEC("fexit/mark_page_accessed")
int kepler_read_page_trace(void *ctx)
{
	u32 curr_tgid;

	curr_tgid = bpf_get_current_pid_tgid() >> 32;
	do_page_cache_hit_increment(curr_tgid);
	return 0;
}

// count write page cache
SEC("tp/writeback_dirty_folio")
int kepler_write_page_trace(void *ctx)
{
	u32 curr_tgid;

	curr_tgid = bpf_get_current_pid_tgid() >> 32;
	do_page_cache_hit_increment(curr_tgid);
	return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
