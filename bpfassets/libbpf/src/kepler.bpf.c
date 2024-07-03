// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#include "kepler.bpf.h"
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, u32);
	__type(value, process_metrics_t);
	__uint(max_entries, MAP_SIZE);
} processes SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__type(key, u32);
	__type(value, u64);
	__uint(max_entries, MAP_SIZE);
} pid_time_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__type(key, int);
	__type(value, u32);
	__uint(max_entries, NUM_CPUS);
} cpu_cycles_event_reader SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, u32);
	__type(value, u64);
	__uint(max_entries, NUM_CPUS);
} cpu_cycles SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__type(key, int);
	__type(value, u32);
	__uint(max_entries, NUM_CPUS);
} cpu_instructions_event_reader SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, u32);
	__type(value, u64);
	__uint(max_entries, NUM_CPUS);
} cpu_instructions SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__type(key, int);
	__type(value, u32);
	__uint(max_entries, NUM_CPUS);
} cache_miss_event_reader SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, u32);
	__type(value, u64);
	__uint(max_entries, NUM_CPUS);
} cache_miss SEC(".maps");

// The sampling rate should be disabled by default because its impact on the
// measurements is unknown.
SEC(".rodata.config")
__attribute__((
	btf_decl_tag("Sample Rate"))) static volatile const int SAMPLE_RATE = 0;

int counter_sched_switch = 0;

static inline u64 calc_delta(u64 *prev_val, u64 val)
{
	u64 delta = 0;
	// Probably a clock issue where the recorded on-CPU event had a
	// timestamp later than the recorded off-CPU event, or vice versa.
	if (prev_val && val > *prev_val)
		delta = val - *prev_val;

	return delta;
}

static inline u64 get_on_cpu_elapsed_time_us(u32 prev_pid, u64 curr_ts)
{
	u64 cpu_time = 0;
	u64 *prev_ts;

	prev_ts = bpf_map_lookup_elem(&pid_time_map, &prev_pid);
	if (prev_ts) {
		cpu_time = calc_delta(prev_ts, curr_ts) / 1000;
		bpf_map_delete_elem(&pid_time_map, &prev_pid);
	}

	return cpu_time;
}

static inline u64 get_on_cpu_cycles(u32 *cpu_id)
{
	u64 delta, val, *prev_val;
	long error;
	struct bpf_perf_event_value c = {};

	error = bpf_perf_event_read_value(
		&cpu_cycles_event_reader, *cpu_id, &c, sizeof(c));
	if (error)
		return 0;

	val = c.counter;
	prev_val = bpf_map_lookup_elem(&cpu_cycles, cpu_id);
	delta = calc_delta(prev_val, val);
	bpf_map_update_elem(&cpu_cycles, cpu_id, &val, BPF_ANY);

	return delta;
}

static inline u64 get_on_cpu_instr(u32 *cpu_id)
{
	u64 delta, val, *prev_val;
	long error;
	struct bpf_perf_event_value c = {};

	error = bpf_perf_event_read_value(
		&cpu_instructions_event_reader, *cpu_id, &c, sizeof(c));
	if (error)
		return 0;

	val = c.counter;
	prev_val = bpf_map_lookup_elem(&cpu_instructions, cpu_id);
	delta = calc_delta(prev_val, val);
	bpf_map_update_elem(&cpu_instructions, cpu_id, &val, BPF_ANY);

	return delta;
}

static inline u64 get_on_cpu_cache_miss(u32 *cpu_id)
{
	u64 delta, val, *prev_val;
	long error;
	struct bpf_perf_event_value c = {};

	error = bpf_perf_event_read_value(
		&cache_miss_event_reader, *cpu_id, &c, sizeof(c));
	if (error)
		return 0;
	val = c.counter;
	prev_val = bpf_map_lookup_elem(&cache_miss, cpu_id);
	delta = calc_delta(prev_val, val);
	bpf_map_update_elem(&cache_miss, cpu_id, &val, BPF_ANY);

	return delta;
}

static inline void register_new_process_if_not_exist()
{
	u64 cgroup_id, pid_tgid;
	u32 curr_tgid;
	struct process_metrics_t *curr_tgid_metrics;

	pid_tgid = bpf_get_current_pid_tgid();
	curr_tgid = pid_tgid >> 32;

	// create new process metrics
	curr_tgid_metrics = bpf_map_lookup_elem(&processes, &curr_tgid);
	if (!curr_tgid_metrics) {
		cgroup_id = bpf_get_current_cgroup_id();
		// the Kernel tgid is the user-space PID, and the Kernel pid is the
		// user-space TID
		process_metrics_t new_process = {
			.pid = curr_tgid,
			.cgroup_id = cgroup_id,
		};
		bpf_get_current_comm(&new_process.comm, sizeof(new_process.comm));
		bpf_map_update_elem(
			&processes, &curr_tgid, &new_process, BPF_NOEXIST);
	}
}

static inline void collect_metrics_and_reset_counters(
	struct process_metrics_t *buf, u32 prev_pid, u64 curr_ts, u32 cpu_id)
{
	buf->cpu_cycles = get_on_cpu_cycles(&cpu_id);
	buf->cpu_instr = get_on_cpu_instr(&cpu_id);
	buf->cache_miss = get_on_cpu_cache_miss(&cpu_id);
	// Get current time to calculate the previous task on-CPU time
	buf->process_run_time = get_on_cpu_elapsed_time_us(prev_pid, curr_ts);
}

struct task_struct {
	int pid;
	unsigned int tgid;
} __attribute__((preserve_access_index));

SEC("tp_btf/sched_switch")
int kepler_sched_switch_trace(u64 *ctx)
{
	u32 prev_pid, next_pid, cpu_id;
	u64 prev_tgid;
	unsigned int prev_state;
	u64 curr_ts = bpf_ktime_get_ns();
	struct task_struct *prev_task, *next_task;

	struct process_metrics_t *curr_tgid_metrics, *prev_tgid_metrics;
	struct process_metrics_t buf = {};

	prev_task = (struct task_struct *)ctx[1];
	next_task = (struct task_struct *)ctx[2];

	prev_pid = (u32)prev_task->pid;
	next_pid = (u32)next_task->pid;
	prev_tgid = prev_task->tgid;
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

	// The process_run_time is 0 if we do not have the previous timestamp of
	// the task or due to a clock issue. In either case, we skip collecting
	// all metrics to avoid discrepancies between the hardware counter and CPU
	// time.
	if (buf.process_run_time > 0) {
		prev_tgid_metrics = bpf_map_lookup_elem(&processes, &prev_tgid);
		if (prev_tgid_metrics) {
			prev_tgid_metrics->process_run_time += buf.process_run_time;
			prev_tgid_metrics->cpu_cycles += buf.cpu_cycles;
			prev_tgid_metrics->cpu_instr += buf.cpu_instr;
			prev_tgid_metrics->cache_miss += buf.cache_miss;
		}
	}

	// Add task on-cpu running start time
	bpf_map_update_elem(&pid_time_map, &next_pid, &curr_ts, BPF_ANY);

	// create new process metrics
	register_new_process_if_not_exist();

	return 0;
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
	struct process_metrics_t *process_metrics;

	curr_tgid = bpf_get_current_pid_tgid() >> 32;
	process_metrics = bpf_map_lookup_elem(&processes, &curr_tgid);
	if (process_metrics)
		process_metrics->page_cache_hit++;
	return 0;
}

// count write page cache
SEC("tp/writeback_dirty_folio")
int kepler_write_page_trace(void *ctx)
{
	u32 curr_tgid;
	struct process_metrics_t *process_metrics;

	curr_tgid = bpf_get_current_pid_tgid() >> 32;
	process_metrics = bpf_map_lookup_elem(&processes, &curr_tgid);
	if (process_metrics)
		process_metrics->page_cache_hit++;
	return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
