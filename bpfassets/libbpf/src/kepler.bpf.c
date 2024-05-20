// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#include "kepler.bpf.h"
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u32);
	__type(value, process_metrics_t);
	__uint(max_entries, MAP_SIZE);
} processes SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__type(key, u32);
	__type(value, u64);
	__uint(max_entries, MAP_SIZE);
} pid_time SEC(".maps");

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

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__type(key, int);
	__type(value, u32);
	__uint(max_entries, NUM_CPUS);
} task_clock_ms_event_reader SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, u32);
	__type(value, u64);
	__uint(max_entries, NUM_CPUS);
} task_clock SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__type(key, u32);
	__type(value, u32);
	__uint(max_entries, NUM_CPUS);
} cpu_freq_array SEC(".maps");

SEC(".rodata.config")
__attribute__((
	btf_decl_tag("Sample Rate"))) static volatile const int SAMPLE_RATE = 5;

SEC(".rodata.config")
__attribute__((btf_decl_tag("CPU Reference Frequency"))) static volatile const int
	CPU_REF_FREQ = 2500;

SEC(".rodata.config")
__attribute__((btf_decl_tag("Hertz Multiplier"))) static volatile const int HZ =
	1000;

int counter_sched_switch = 0;

static inline u64 get_on_cpu_time(u32 cur_pid, u32 prev_pid, u64 cur_ts)
{
	u64 cpu_time = 0;
	pid_time_t prev_pid_key = { .pid = prev_pid };
	pid_time_t new_pid_key = { .pid = cur_pid };

	u64 *prev_ts = bpf_map_lookup_elem(&pid_time, &prev_pid_key);
	if (prev_ts) {
		// Probably a clock issue where the recorded on-CPU event had a
		// timestamp later than the recorded off-CPU event, or vice versa.
		if (cur_ts > *prev_ts) {
			cpu_time = (cur_ts - *prev_ts) / 1000000; // convert to ms
			bpf_map_delete_elem(&pid_time, &prev_pid_key);
		}
	}

	bpf_map_update_elem(&pid_time, &new_pid_key, &cur_ts, BPF_NOEXIST);
	return cpu_time;
}

static inline u64 calc_delta(u64 *prev_val, u64 val)
{
	u64 delta = 0;
	if (prev_val && val > *prev_val) {
		delta = val - *prev_val;
	}

	return delta;
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
	if (error) {
		return 0;
	}
	val = c.counter;
	prev_val = bpf_map_lookup_elem(&cache_miss, cpu_id);
	delta = calc_delta(prev_val, val);
	bpf_map_update_elem(&cache_miss, cpu_id, &val, BPF_ANY);

	return delta;
}

// This struct is defined according to the following format file:
// /sys/kernel/tracing/events/sched/sched_switch/format
struct sched_switch_info {
	/* The first 8 bytes is not allowed to read */
	unsigned long pad;

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
	u32 prev_pid, tgid, cpu_id;
	u64 pid_tgid, cgroup_id, cur_ts;
	pid_t cur_pid;

	struct process_metrics_t *cur_pid_metrics, *prev_pid_metrics;
	struct process_metrics_t buf = {};

	if (SAMPLE_RATE > 0) {
		if (counter_sched_switch > 0) {
			counter_sched_switch--;
			return 0;
		}
		counter_sched_switch = SAMPLE_RATE;
	}

	prev_pid = ctx->prev_pid;
	pid_tgid = bpf_get_current_pid_tgid();
	cur_pid = pid_tgid & 0xffffffff;
	tgid = pid_tgid >> 32;
	cgroup_id = bpf_get_current_cgroup_id();
	cpu_id = bpf_get_smp_processor_id();
	cur_ts = bpf_ktime_get_ns();
	buf.cpu_cycles = get_on_cpu_cycles(&cpu_id);
	buf.cpu_instr = get_on_cpu_instr(&cpu_id);
	buf.cache_miss = get_on_cpu_cache_miss(&cpu_id);
	buf.process_run_time = get_on_cpu_time(cur_pid, prev_pid, cur_ts);
	buf.task_clock_time = buf.cpu_cycles / 1000000; // convert to ms

	prev_pid_metrics = bpf_map_lookup_elem(&processes, &prev_pid);
	if (prev_pid_metrics) {
		// update process time
		prev_pid_metrics->process_run_time += buf.process_run_time;
		prev_pid_metrics->task_clock_time += buf.task_clock_time;
		prev_pid_metrics->cpu_cycles += buf.cpu_cycles;
		prev_pid_metrics->cpu_instr += buf.cpu_instr;
		prev_pid_metrics->cache_miss += buf.cache_miss;
	}

	// create new process metrics
	cur_pid_metrics = bpf_map_lookup_elem(&processes, &cur_pid);
	if (!cur_pid_metrics) {
		process_metrics_t new_process = {
			.pid = cur_pid,
			.tgid = tgid,
			.cgroup_id = cgroup_id,
		};
		bpf_get_current_comm(&new_process.comm, sizeof(new_process.comm));
		bpf_map_update_elem(
			&processes, &cur_pid, &new_process, BPF_NOEXIST);
	}

	return 0;
}

// This struct is defined according to the following format file:
//  /sys/kernel/tracing/events/irq/softirq_entry/format
struct trace_event_raw_softirq {
	/* The first 8 bytes is not allowed to read */
	unsigned long pad;
	unsigned int vec;
};

SEC("tp/irq/softirq_entry")
int kepler_irq_trace(struct trace_event_raw_softirq *ctx)
{
	u32 cur_pid;
	struct process_metrics_t *process_metrics;
	unsigned int vec;

	cur_pid = bpf_get_current_pid_tgid();
	vec = ctx->vec;
	process_metrics = bpf_map_lookup_elem(&processes, &cur_pid);
	if (process_metrics != 0) {
		if (vec < 10) {
			u16 count = process_metrics->vec_nr[vec];
			count++;
			process_metrics->vec_nr[vec] = count;
		}
	}
	return 0;
}

// count read page cache
SEC("fexit/mark_page_accessed")
int kepler_read_page_trace(void *ctx)
{
	u32 cur_pid;
	struct process_metrics_t *process_metrics;

	cur_pid = bpf_get_current_pid_tgid();
	process_metrics = bpf_map_lookup_elem(&processes, &cur_pid);
	if (process_metrics) {
		process_metrics->page_cache_hit++;
	}
	return 0;
}

// count write page cache
SEC("tp/writeback/writeback_dirty_folio")
int kepler_write_page_trace(void *ctx)
{
	u32 cur_pid;
	struct process_metrics_t *process_metrics;

	cur_pid = bpf_get_current_pid_tgid();
	process_metrics = bpf_map_lookup_elem(&processes, &cur_pid);
	if (process_metrics) {
		process_metrics->page_cache_hit++;
	}
	return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
