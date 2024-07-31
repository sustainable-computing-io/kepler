// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#include "kepler.bpf.h"

// Ring buffer sizing
// 256kB is sufficient to store around 1000 events/sec for 5 seconds
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 256 * 1024); // 256 KB
} rb SEC(".maps");
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__type(key, int);
	__type(value, u32);
} cpu_cycles_event_reader SEC(".maps");
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__type(key, int);
	__type(value, u32);
} cpu_instructions_event_reader SEC(".maps");
struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__type(key, int);
	__type(value, u32);
} cache_miss_event_reader SEC(".maps");

SEC(".rodata.config")
__attribute__((btf_decl_tag("Hardware Events Enabled"))) volatile const int HW = 1;

static __always_inline u64 get_on_cpu_cycles(u32 *cpu_id)
{
	long error;
	struct bpf_perf_event_value c = {};

	error = bpf_perf_event_read_value(
		&cpu_cycles_event_reader, *cpu_id, &c, sizeof(c));
	if (error)
		return 0;

	return c.counter;
}

static __always_inline u64 get_on_cpu_instr(u32 *cpu_id)
{
	long error;
	struct bpf_perf_event_value c = {};

	error = bpf_perf_event_read_value(
		&cpu_instructions_event_reader, *cpu_id, &c, sizeof(c));
	if (error)
		return 0;

	return c.counter;
}

static __always_inline u64 get_on_cpu_cache_miss(u32 *cpu_id)
{
	long error;
	struct bpf_perf_event_value c = {};

	error = bpf_perf_event_read_value(
		&cache_miss_event_reader, *cpu_id, &c, sizeof(c));
	if (error)
		return 0;

	return c.counter;
}

// Wake up userspace if there are at least 1000 events unprocessed
const long wakeup_data_size = sizeof(struct event) * 1000;

// Get the flags for the ring buffer submit
static inline long get_flags()
{
	long sz;

	if (!wakeup_data_size)
		return 0;

	sz = bpf_ringbuf_query(&rb, BPF_RB_AVAIL_DATA);
	return sz >= wakeup_data_size ? BPF_RB_FORCE_WAKEUP : BPF_RB_NO_WAKEUP;
}

static inline int do_kepler_sched_switch_trace(
	u32 prev_pid, u32 prev_tgid, u32 next_pid, u32 next_tgid)
{
	struct event *e;
	u64 cpu_cycles, cpu_instr, cache_miss = 0;

	e = bpf_ringbuf_reserve(&rb, sizeof(*e), 0);
	if (!e)
		return 0;

	e->ts = bpf_ktime_get_ns();
	e->event_type = SCHED_SWITCH;
	e->cpu_id = bpf_get_smp_processor_id();
	e->pid = next_tgid;
	e->tid = next_pid;
	e->offcpu_pid = prev_tgid;
	e->offcpu_tid = prev_pid;
	if (HW) {
		e->cpu_cycles = get_on_cpu_cycles(&e->cpu_id);
		e->cpu_instr = get_on_cpu_instr(&e->cpu_id);
		e->cache_miss = get_on_cpu_cache_miss(&e->cpu_id);
	}
	e->offcpu_cgroup_id = bpf_get_current_cgroup_id();

	bpf_ringbuf_submit(e, get_flags());

	return 0;
}

static inline int do_kepler_irq_trace(u32 vec)
{
	struct event *e;

	// We are interested in NET_TX, NET_RX, and BLOCK
	if (vec == NET_TX || vec == NET_RX || vec == BLOCK) {
		e = bpf_ringbuf_reserve(&rb, sizeof(*e), 0);
		if (!e)
			return 0;
		e->event_type = IRQ;
		e->ts = bpf_ktime_get_ns();
		e->cpu_id = bpf_get_smp_processor_id();
		e->pid = bpf_get_current_pid_tgid() >> 32;
		e->tid = (u32)bpf_get_current_pid_tgid();
		e->irq_number = vec;

		bpf_ringbuf_submit(e, get_flags());
	}

	return 0;
}

static inline int do_page_cache_hit_increment(u32 curr_tgid)
{
	struct event *e;

	e = bpf_ringbuf_reserve(&rb, sizeof(*e), 0);
	if (!e)
		return 0;
	e->event_type = PAGE_CACHE_HIT;
	e->ts = bpf_ktime_get_ns();
	e->pid = curr_tgid;

	bpf_ringbuf_submit(e, get_flags());

	return 0;
}

static inline int do_process_free(u32 curr_tgid)
{
	struct event *e;

	e = bpf_ringbuf_reserve(&rb, sizeof(*e), 0);
	if (!e)
		return 0;
	e->event_type = FREE;
	e->ts = bpf_ktime_get_ns();
	e->pid = curr_tgid;

	bpf_ringbuf_submit(e, get_flags());

	return 0;
}

SEC("tp_btf/sched_switch")
int kepler_sched_switch_trace(u64 *ctx)
{
	struct task_struct *prev_task, *next_task;

	prev_task = (struct task_struct *)ctx[1];
	next_task = (struct task_struct *)ctx[2];

	return do_kepler_sched_switch_trace(
		prev_task->pid, prev_task->tgid, next_task->pid, next_task->tgid);
}

SEC("tp_btf/softirq_entry")
int kepler_irq_trace(u64 *ctx)
{
	unsigned int vec;
	vec = (unsigned int)ctx[0];

	do_kepler_irq_trace(vec);

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

SEC("tp_btf/sched_process_free")
int kepler_sched_process_free(u64 *ctx)
{
	struct task_struct *task;
	task = (struct task_struct *)ctx[0];
	do_process_free(task->tgid);
	return 0;
}

// TEST PROGRAMS - These programs are never attached in production

SEC("raw_tp")
int test_kepler_write_page_trace(void *ctx)
{
	do_page_cache_hit_increment(42);
	return 0;
}

SEC("raw_tp")
int test_kepler_sched_switch_trace(u64 *ctx)
{
	// 42 going offcpu, 43 going on cpu
	do_kepler_sched_switch_trace(42, 42, 43, 43);
	return 0;
}

SEC("raw_tp")
int test_kepler_sched_process_free(u64 *ctx)
{
	do_process_free(42);
	return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
