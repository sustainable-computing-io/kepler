
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

typedef struct cgroup_time_t
{
    u64 cgroup_id;
    u64 time;
    u64 cpu_cycles;
    u64 cpu_instr;
    u64 cache_misses;
    char comm[16];
} cgroup_time_t;

typedef struct pid_time_t
{
    int pid;
} pid_time_t;

BPF_PERF_OUTPUT(events);

// cgroup and pid time
BPF_HASH(cgroups, u64, cgroup_time_t);
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

int sched_switch(switch_args *ctx)
{
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
        delta = (time - *last_time) / 1000;
        pid_time.delete(&old_pid);
    }

    new_pid.pid = ctx->next_pid;
    pid_time.update(&new_pid, &time);

    u64 cpu_cycles_delta = 0;
    u64 cpu_instr_delta = 0;
    u64 cache_miss_delta = 0;
    u64 *prev;

    u64 val = cpu_cycles.perf_read(CUR_CPU_IDENTIFIER);
    prev = prev_cpu_cycles.lookup(&cpu);
    if (prev)
    {
        cpu_cycles_delta = val - *prev;
    }
    else
        prev_cpu_cycles.update(&cpu, &val);

    val = cpu_instr.perf_read(CUR_CPU_IDENTIFIER);
    prev = prev_cpu_instr.lookup(&cpu);
    if (prev)
    {
        cpu_instr_delta = val - *prev;
    }
    prev_cpu_instr.update(&cpu, &val);

    val = cache_miss.perf_read(CUR_CPU_IDENTIFIER);
    prev = prev_cache_miss.lookup(&cpu);
    if (prev)
    {
        cache_miss_delta = val - *prev;
    }
    prev_cache_miss.update(&cpu, &val);

    // update cgroup time
    cgroup_time_t new_cgroup;
    new_cgroup.cgroup_id = cgroup_id;
    new_cgroup.time = delta;
    new_cgroup.cpu_cycles = cpu_cycles_delta;
    new_cgroup.cpu_instr = cpu_instr_delta;
    new_cgroup.cache_misses = cache_miss_delta;
    bpf_get_current_comm(&new_cgroup.comm, sizeof(new_cgroup.comm));

    cgroup_time_t *cgroup_time = cgroups.lookup_or_init(&cgroup_id, &new_cgroup);
    cgroup_time->time += delta;
    cgroup_time->cpu_cycles += cpu_cycles_delta;
    cgroup_time->cpu_instr += cpu_instr_delta;
    cgroup_time->cache_misses += cache_miss_delta;
    bpf_get_current_comm(&cgroup_time->comm, sizeof(cgroup_time->comm));
    cgroups.update(&cgroup_id, cgroup_time);

    return 0;
}