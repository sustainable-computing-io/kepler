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

#include <uapi/linux/ptrace.h>
#include <uapi/linux/bpf_perf_event.h>

typedef struct switch_args
{
    u64 pad;
    char prev_comm[16];
    int prev_pid;
    int prev_prio;
    long long prev_state;
    char next_comm[16];
    int next_pid;
    int next_prio;
} switch_args;

typedef struct cpu_freq_args
{
    u64 pad;
    u32 state;
    u32 cpu_id;
} cpu_freq_args;

typedef struct process_time_t
{
    u64 cgroup_id;
    u64 pid;
    u64 time;
    u64 cpu_cycles;
    u64 cpu_instr;
    u64 cache_misses;
    u64 start_time;
    u64 last_avg_freq_update_time;
    u32 avg_freq;
    u32 last_freq;
    char comm[16];
} process_time_t;

typedef struct pid_time_t
{
    int pid;
} pid_time_t;

BPF_PERF_OUTPUT(events);

// processes and pid time
BPF_HASH(processes, u64, process_time_t);
BPF_HASH(pid_time, pid_time_t);

#ifndef NUM_CPUS
#define NUM_CPUS 128
#endif

// perf counters
BPF_PERF_ARRAY(cpu_cycles, NUM_CPUS);
BPF_PERF_ARRAY(cpu_instr, NUM_CPUS);
BPF_PERF_ARRAY(cache_miss, NUM_CPUS);

// tracking counters
BPF_ARRAY(prev_cpu_cycles, u64, NUM_CPUS);
BPF_ARRAY(prev_cpu_instr, u64, NUM_CPUS);
BPF_ARRAY(prev_cache_miss, u64, NUM_CPUS);

// cpu freq counters
BPF_ARRAY(cpu_freq_array, u32, NUM_CPUS);

int sched_switch(switch_args *ctx)
{
    u64 pid = bpf_get_current_pid_tgid() >> 32;

    u64 time = bpf_ktime_get_ns();
    u64 delta = 0;
    u32 cpu = bpf_get_smp_processor_id();
    u64 cgroup_id = bpf_get_current_cgroup_id();
    pid_time_t new_pid, old_pid;

    // get pid time
    old_pid.pid = ctx->prev_pid;
    u64 *last_time = pid_time.lookup(&old_pid);
    if (last_time != 0)
    {
        delta = (time - *last_time) / 1000; /*microsecond*/
        // return if the process did not use any cpu time yet
        if (delta == 0)
        {
            return 0;
        }
        pid_time.delete(&old_pid);
    }

    new_pid.pid = ctx->next_pid;
    pid_time.update(&new_pid, &time);

    u64 cpu_cycles_delta = 0;
    u64 cpu_instr_delta = 0;
    u64 cache_miss_delta = 0;
    u64 *prev;

    u64 val = cpu_cycles.perf_read(CUR_CPU_IDENTIFIER);
    if (((s64)val > 0) || ((s64)val < -256))
    {
        prev = prev_cpu_cycles.lookup(&cpu);
        if (prev)
        {
            cpu_cycles_delta = val - *prev;
        }
        prev_cpu_cycles.update(&cpu, &val);
    }
    val = cpu_instr.perf_read(CUR_CPU_IDENTIFIER);
    if (((s64)val > 0) || ((s64)val < -256))
    {
        prev = prev_cpu_instr.lookup(&cpu);
        if (prev)
        {
            cpu_instr_delta = val - *prev;
        }
        prev_cpu_instr.update(&cpu, &val);
    }
    val = cache_miss.perf_read(CUR_CPU_IDENTIFIER);
    if (((s64)val > 0) || ((s64)val < -256))
    {
        prev = prev_cache_miss.lookup(&cpu);
        if (prev)
        {
            cache_miss_delta = val - *prev;
        }
        prev_cache_miss.update(&cpu, &val);
    }

    // get cpu freq 
    u32 last_freq = 0; // if no cpu frequency found, use this one
    u32 init_freq = 10; //use a small init freq to start it off
    u32 *freq = cpu_freq_array.lookup(&cpu);
    if (freq && *freq > init_freq) {
        last_freq = *freq;
    }else{
        cpu_freq_array.update(&cpu, &init_freq);
    }
 
    // init process time
    struct process_time_t *process_time;
    process_time = processes.lookup(&pid);
    if (process_time == 0)
    {
        process_time_t new_process = {};
        new_process.pid = pid;
        new_process.time = delta;
        new_process.cpu_cycles = cpu_cycles_delta;
        new_process.cpu_instr = cpu_instr_delta;
        new_process.cache_misses = cache_miss_delta;
        bpf_get_current_comm(&new_process.comm, sizeof(new_process.comm));
        new_process.start_time = time;
        new_process.last_freq = last_freq;
        new_process.last_avg_freq_update_time = time;
        new_process.avg_freq = last_freq;
        new_process.cgroup_id = cgroup_id;
        processes.update(&pid, &new_process);
    }
    else
    {
        // update process time
        process_time->time += delta;
        process_time->cpu_cycles += cpu_cycles_delta;
        process_time->cpu_instr += cpu_instr_delta;
        process_time->cache_misses += cache_miss_delta;

        // calculate runtime cpu frequency average
        process_time->last_freq = last_freq;        
        u64 last_freq_total_weight = (process_time->last_avg_freq_update_time - process_time->start_time)* process_time->avg_freq;
        u64 freq_time_delta = time - process_time->last_avg_freq_update_time;
        u64 last_freq_weight = process_time->last_freq * freq_time_delta;
        process_time->avg_freq = (u32)((last_freq_total_weight + last_freq_weight)/(time - process_time->start_time));
        process_time->last_avg_freq_update_time = time;        
    }

    return 0;
}

int cpu_freq(cpu_freq_args *ctx)
{
    u32 cpu = ctx->cpu_id;
    u32 state = ctx->state;

    cpu_freq_array.update(&cpu, &state);
    return 0;
}