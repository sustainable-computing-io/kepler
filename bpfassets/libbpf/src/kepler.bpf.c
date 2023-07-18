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

// +build ignore

#include <linux/version.h>

#if (LINUX_KERNEL_VERSION >= KERNEL_VERSION(5, 12, 0))
#define BPF_PERF_EVENT_READ_VALUE_AVAILABLE 1
#endif

#include "kepler.bpf.h"

// processes and pid time
BPF_HASH(processes, u64, process_metrics_t);
BPF_HASH(pid_time, pid_time_t, u64);

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

static inline u64 get_on_cpu_time(u32 cur_pid, u32 prev_pid, u32 cpu_id, u64 cur_ts)
{
    u64 cpu_time = 0;

    // get pid time
    pid_time_t prev_pid_key = {.pid = prev_pid, .cpu = cpu_id};
    u64 *prev_ts;
    prev_ts = bpf_map_lookup_elem(&pid_time, &prev_pid_key);
    if (prev_ts)
    {
        // Probably a clock issue where the recorded on-CPU event had a
        // timestamp later than the recorded off-CPU event, or vice versa.
        // But do not return, since the hardware counters can be collected.
        if (cur_ts > *prev_ts)
        {
            cpu_time = (cur_ts - *prev_ts) / 1000; /*milisecond*/
            bpf_map_delete_elem(&pid_time, &prev_pid_key);
        }
    }
    pid_time_t new_pid_key = {.pid = cur_pid, .cpu = cpu_id};
    bpf_map_update_elem(&pid_time, &new_pid_key, &cur_ts, BPF_NOEXIST);

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

// although the "get_on_cpu_counters" has some code duplications, it is inline code and the compiles will improve this
static inline u64 get_on_cpu_cycles(u32 *cpu_id)
{
    u64 delta = 0;
#ifdef BPF_PERF_EVENT_READ_VALUE_AVAILABLE
    struct bpf_perf_event_value c = {};
    int error = bpf_perf_event_read_value(&cpu_cycles_hc_reader, *cpu_id, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = bpf_map_lookup_elem(&cpu_cycles, cpu_id);
        delta = calc_delta(prev_val, &val);
        bpf_map_update_elem(&cpu_cycles, cpu_id, &val, BPF_ANY);
    }
#else
    int ret = bpf_perf_event_read(&cpu_cycles_hc_reader, *cpu_id);
    if (ret < 0) {
        return delta;
    }
    u64 val = ret;
    u64 *prev_val = bpf_map_lookup_elem(&cpu_cycles, cpu_id);
    delta = calc_delta(prev_val, &val);
    bpf_map_update_elem(&cpu_cycles, cpu_id, &val, BPF_ANY);
#endif

    return delta;
}

static inline u64 get_on_cpu_ref_cycles(u32 *cpu_id)
{
    u64 delta = 0;
#ifdef BPF_PERF_EVENT_READ_VALUE_AVAILABLE
    struct bpf_perf_event_value c = {};
    int error = bpf_perf_event_read_value(&cpu_ref_cycles_hc_reader, *cpu_id, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = bpf_map_lookup_elem(&cpu_ref_cycles, cpu_id);
        delta = calc_delta(prev_val, &val);
        bpf_map_update_elem(&cpu_ref_cycles, cpu_id, &val, BPF_ANY);
    }
#else
    int ret = bpf_perf_event_read(&cpu_ref_cycles_hc_reader, *cpu_id);
    if (ret < 0) {
        return delta;
    }
    u64 val = ret;
    u64 *prev_val = bpf_map_lookup_elem(&cpu_ref_cycles, cpu_id);
    delta = calc_delta(prev_val, &val);
    bpf_map_update_elem(&cpu_ref_cycles, cpu_id, &val, BPF_ANY);
#endif
    return delta;
}

static inline u64 get_on_cpu_instr(u32 *cpu_id)
{
    u64 delta = 0;
#ifdef BPF_PERF_EVENT_READ_VALUE_AVAILABLE
    struct bpf_perf_event_value c = {};
    int error = bpf_perf_event_read_value(&cpu_instr_hc_reader, *cpu_id, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = bpf_map_lookup_elem(&cpu_instr, cpu_id);
        delta = calc_delta(prev_val, &val);
        bpf_map_update_elem(&cpu_instr, cpu_id, &val, BPF_ANY);
    }
#else
    int ret = bpf_perf_event_read(&cpu_instr_hc_reader, *cpu_id);
    if (ret < 0) {
        return delta;
    }
    u64 val = ret;
    u64 *prev_val = bpf_map_lookup_elem(&cpu_instr, cpu_id);
    delta = calc_delta(prev_val, &val);
    bpf_map_update_elem(&cpu_instr, cpu_id, &val, BPF_ANY);
#endif
    return delta;
}

static inline u64 get_on_cpu_cache_miss(u32 *cpu_id)
{
    u64 delta = 0;
#ifdef BPF_PERF_EVENT_READ_VALUE_AVAILABLE
    struct bpf_perf_event_value c = {};
    int error = bpf_perf_event_read_value(&cache_miss_hc_reader, *cpu_id, &c, sizeof(struct bpf_perf_event_value));
    if (error == 0)
    {
        u64 val = normalize(&c.counter, &c.enabled, &c.running);
        u64 *prev_val = bpf_map_lookup_elem(&cache_miss, cpu_id);
        delta = calc_delta(prev_val, &val);
        bpf_map_update_elem(&cache_miss, cpu_id, &val, BPF_ANY);
    }
#else
    int ret = bpf_perf_event_read(&cache_miss_hc_reader, *cpu_id);
    if (ret < 0) {
        return delta;
    }
    u64 val = ret;
    u64 *prev_val = bpf_map_lookup_elem(&cache_miss, cpu_id);
    delta = calc_delta(prev_val, &val);
    bpf_map_update_elem(&cache_miss, cpu_id, &val, BPF_ANY);
#endif
    return delta;
}

// calculate the average cpu freq
static inline u64 get_on_cpu_avg_freq(u32 *cpu_id, u64 on_cpu_cycles_delta, u64 on_cpu_ref_cycles_delta)
{
    u32 avg_freq = 0;
    bpf_map_lookup_or_try_init(&cpu_freq_array, cpu_id, &avg_freq);
    if (avg_freq == 0)
    {
        avg_freq = ((on_cpu_cycles_delta * CPU_REF_FREQ) / on_cpu_ref_cycles_delta) * HZ;
    }
    else
    {
        avg_freq += ((on_cpu_cycles_delta * CPU_REF_FREQ) / on_cpu_ref_cycles_delta) * HZ;
        avg_freq /= 2;
    }
    bpf_map_update_elem(&cpu_freq_array, cpu_id, &avg_freq, BPF_ANY);
    return avg_freq;
}

SEC("tracepoint/sched/sched_switch")
int kepler_trace(struct sched_switch_args *ctx)
{
    u64 cur_pid = bpf_get_current_pid_tgid() >> 32;
    u64 cgroup_id = bpf_get_current_cgroup_id();
    u64 cur_ts = bpf_ktime_get_ns();
    u32 cpu_id = bpf_get_smp_processor_id();
    u64 prev_pid = ctx->prev_pid;
    u64 on_cpu_time_delta = get_on_cpu_time(cur_pid, prev_pid, cpu_id, cur_ts);
    u64 on_cpu_cycles_delta = get_on_cpu_cycles(&cpu_id);
    u64 on_cpu_ref_cycles_delta = get_on_cpu_ref_cycles(&cpu_id);
    u64 on_cpu_instr_delta = get_on_cpu_instr(&cpu_id);
    u64 on_cpu_cache_miss_delta = get_on_cpu_cache_miss(&cpu_id);
    u64 on_cpu_avg_freq = get_on_cpu_avg_freq(&cpu_id, on_cpu_cycles_delta, on_cpu_ref_cycles_delta);

    // store process metrics
    struct process_metrics_t *process_metrics;
    process_metrics = bpf_map_lookup_elem(&processes, &prev_pid);
    if (process_metrics)
    {
        // update process time
        process_metrics->process_run_time += on_cpu_time_delta;

        process_metrics->cpu_cycles += on_cpu_cycles_delta;
        process_metrics->cpu_instr += on_cpu_instr_delta;
        process_metrics->cache_miss += on_cpu_cache_miss_delta;
    }

    process_metrics = bpf_map_lookup_elem(&processes, &cur_pid);
    if (process_metrics == 0)
    {
        process_metrics_t new_process = {};
        new_process.pid = cur_pid;
        new_process.cgroup_id = cgroup_id;
        bpf_get_current_comm(&new_process.comm, sizeof(new_process.comm));
        bpf_map_update_elem(&processes, &cur_pid, &new_process, BPF_NOEXIST);
    }
    return 0;
}

SEC("tracepoint/irq/softirq_entry")
int kepler_irq_trace(struct trace_event_raw_softirq *ctx)
{
    u64 cur_pid = bpf_get_current_pid_tgid() >> 32;
    struct process_metrics_t *process_metrics;
    process_metrics = bpf_map_lookup_elem(&processes, &cur_pid);
    if (process_metrics != 0)
    {
        if (ctx->vec < 10) {
            process_metrics->vec_nr[ctx->vec] ++;
        }
    }
    return 0;
}

char _license[] SEC("license") = "GPL";