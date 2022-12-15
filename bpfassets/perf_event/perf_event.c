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

#include <linux/sched.h>
// #include <linux/bpf.h>
// #include <linux/bpf_perf_event.h>

#ifndef NUM_CPUS
#define NUM_CPUS 128
#endif

#ifndef CPU_REF_FREQ
#define CPU_REF_FREQ 2500
#endif

#define HZ 1000

typedef struct process_metrics_t
{
    u64 cgroup_id;
    u64 pid;
    u64 process_run_time;
    u64 cpu_cycles;
    u64 cpu_instr;
    u64 cache_miss;
    char comm[16];
} process_metrics_t;

typedef struct pid_time_t
{
    u32 pid;
    u32 cpu;
} pid_time_t;

// processes and pid time
BPF_HASH(processes, u64, process_metrics_t);
BPF_HASH(pid_time, pid_time_t);

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

static inline u64 get_on_cpu_time(u32 cur_pid, u32 prev_pid, u64 cur_ts)
{
    u64 cpu_time = 0;

    // get pid time
    pid_time_t prev_pid_key = {.pid = prev_pid};
    u64 *prev_ts = pid_time.lookup(&prev_pid_key);
    if (prev_ts != 0)
    {
        // Probably a clock issue where the recorded on-CPU event had a
        // timestamp later than the recorded off-CPU event, or vice versa.
        // But do not return, since the hardware counters can be collected.
        if (cur_ts > *prev_ts)
        {
            cpu_time = (cur_ts - *prev_ts) / 1000000; /*milisecond*/
            pid_time.delete(&prev_pid_key);
        }
    }
    pid_time_t new_pid_key = {.pid = cur_pid};
    pid_time.update(&new_pid_key, &cur_ts);

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
    u64 cur_pid = bpf_get_current_pid_tgid() >> 32;
#ifdef SET_GROUP_ID
    u64 cgroup_id = bpf_get_current_cgroup_id();
#else
    u64 cgroup_id = 0;
#endif

    u64 cur_ts = bpf_ktime_get_ns();
    u32 cpu_id = bpf_get_smp_processor_id();
    u64 on_cpu_time_delta = get_on_cpu_time(cur_pid, prev->pid, cur_ts);
    u64 on_cpu_cycles_delta = get_on_cpu_cycles(&cpu_id);
    u64 on_cpu_ref_cycles_delta = get_on_cpu_ref_cycles(&cpu_id);
    u64 on_cpu_instr_delta = get_on_cpu_instr(&cpu_id);
    u64 on_cpu_cache_miss_delta = get_on_cpu_cache_miss(&cpu_id);
    u64 on_cpu_avg_freq = get_on_cpu_avg_freq(&cpu_id, on_cpu_cycles_delta, on_cpu_ref_cycles_delta);

    // store process metrics
    struct process_metrics_t *process_metrics;
    process_metrics = processes.lookup(&cur_pid);
    if (process_metrics == 0)
    {
        process_metrics_t new_process = {};
        new_process.pid = cur_pid;
        new_process.cgroup_id = cgroup_id;
        new_process.process_run_time = on_cpu_time_delta;
        bpf_get_current_comm(&new_process.comm, sizeof(new_process.comm));

        new_process.cpu_cycles = on_cpu_cycles_delta;
        new_process.cpu_instr = on_cpu_instr_delta;
        new_process.cache_miss = on_cpu_cache_miss_delta;

        processes.update(&cur_pid, &new_process);
    }
    else
    {
        // update process time
        process_metrics->process_run_time += on_cpu_time_delta;

        process_metrics->cpu_cycles += on_cpu_cycles_delta;
        process_metrics->cpu_instr += on_cpu_instr_delta;
        process_metrics->cache_miss += on_cpu_cache_miss_delta;
    }

    return 0;
}
