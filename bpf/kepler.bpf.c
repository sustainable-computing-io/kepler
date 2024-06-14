// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#include "kepler.bpf.h"

// The sampling rate should be disabled by default because its impact on the
// measurements is unknown.
SEC(".rodata.config")
__attribute__((
	btf_decl_tag("Sample Rate"))) static volatile const int SAMPLE_RATE = 0;

int counter_sched_switch = 0;

static inline void collect_metrics_and_reset_counters(
	struct process_metrics_t *buf, u32 prev_pid, u64 curr_ts, u32 cpu_id)
{
	buf->cpu_cycles = get_on_cpu_cycles(&cpu_id);
	buf->cpu_instr = get_on_cpu_instr(&cpu_id);
	buf->cache_miss = get_on_cpu_cache_miss(&cpu_id);
	// Get current time to calculate the previous task on-CPU time
	buf->process_run_time = get_on_cpu_elapsed_time_us(prev_pid, curr_ts);
}

// This struct is defined according to the following format file:
// /sys/kernel/tracing/events/sched/sched_switch/format
struct sched_switch_info {
	/* The first 8 bytes is not allowed to read */
	u64 pad;

	char prev_comm[16];
	pid_t prev_pid;
	int prev_prio;
	long prev_state;
	char next_comm[16];
	pid_t next_pid;
	int next_prio;
};

SEC("tp/sched/sched_switch")
int kepler_sched_switch_trace(struct sched_switch_info *ctx)
{
	u32 prev_pid, next_pid, cpu_id, curr_pid, curr_tgid;
	u64 *prev_tgid, pid_tgid;
	long prev_state;
	u64 curr_ts = bpf_ktime_get_ns();

	struct process_metrics_t *curr_tgid_metrics, *prev_tgid_metrics;
	struct process_metrics_t buf = {};

	prev_state = ctx->prev_state;
	prev_pid = (u32)ctx->prev_pid;
	next_pid = (u32)ctx->next_pid;
	cpu_id = bpf_get_smp_processor_id();

	// Collect metrics
	// Regardless of skipping the collection, we need to update the hardware
	// counter events to keep the metrics map current.
	collect_metrics_and_reset_counters(&buf, prev_pid, curr_ts, cpu_id);

	// Skip some samples to minimize overhead
	// Note that we can only skip samples after updating the metric maps to
	// collect the right values
	if (SAMPLE_RATE > 0) {
		if (counter_sched_switch > 0) {
			counter_sched_switch--;
			return 0;
		}
		counter_sched_switch = SAMPLE_RATE;
	}

	if (prev_state == TASK_RUNNING) {
		// Skip if the previous thread was not registered yet
		prev_tgid = bpf_map_lookup_elem(&pid_tgid_map, &prev_pid);
		if (prev_tgid) {
			// The process_run_time is 0 if we do not have the previous timestamp of
			// the task or due to a clock issue. In either case, we skip collecting
			// all metrics to avoid discrepancies between the hardware counter and CPU
			// time.
			if (buf.process_run_time > 0) {
				prev_tgid_metrics = bpf_map_lookup_elem(
					&processes, prev_tgid);
				if (prev_tgid_metrics) {
					prev_tgid_metrics->process_run_time +=
						buf.process_run_time;
					prev_tgid_metrics->cpu_cycles +=
						buf.cpu_cycles;
					prev_tgid_metrics->cpu_instr +=
						buf.cpu_instr;
					prev_tgid_metrics->cache_miss +=
						buf.cache_miss;
				}
			}
		}
	}

	// Add task on-cpu running start time
	bpf_map_update_elem(&pid_time_map, &next_pid, &curr_ts, BPF_ANY);

	pid_tgid = bpf_get_current_pid_tgid();
	curr_pid = (u32)pid_tgid;
	curr_tgid = pid_tgid >> 32;

	// create new process metrics
	register_new_process_if_not_exist(curr_pid, curr_tgid);

	return 0;
}

// This struct is defined according to the following format file:
//  /sys/kernel/tracing/events/irq/softirq_entry/format
struct trace_event_raw_softirq {
	/* The first 8 bytes is not allowed to read */
	u64 pad;
	unsigned int vec;
};

SEC("tp/irq/softirq_entry")
int kepler_irq_trace(struct trace_event_raw_softirq *ctx)
{
	u32 curr_pid;
	struct process_metrics_t *process_metrics;
	unsigned int vec;

	curr_pid = bpf_get_current_pid_tgid();
	vec = ctx->vec;
	process_metrics = bpf_map_lookup_elem(&processes, &curr_pid);
	if (process_metrics != 0 && vec < 10)
		process_metrics->vec_nr[vec] += 1;
	return 0;
}

// count read page cache
SEC("fexit/mark_page_accessed")
int kepler_read_page_trace(void *ctx)
{
	u32 curr_pid;

	curr_pid = bpf_get_current_pid_tgid();
	do_page_cache_hit_increment(curr_pid);
	return 0;
}

// count write page cache
SEC("tp/writeback/writeback_dirty_folio")
int kepler_write_page_trace(void *ctx)
{
	u32 curr_pid;

	curr_pid = bpf_get_current_pid_tgid();
	do_page_cache_hit_increment(curr_pid);
	return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
