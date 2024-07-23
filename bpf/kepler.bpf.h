// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#pragma once

typedef unsigned char __u8;
typedef short int __s16;
typedef short unsigned int __u16;
typedef int __s32;
typedef unsigned int __u32;
typedef long long int __s64;
typedef long long unsigned int __u64;
typedef __u8 u8;
typedef __s16 s16;
typedef __u16 u16;
typedef __s32 s32;
typedef __u32 u32;
typedef __s64 s64;
typedef __u64 u64;
typedef __u16 __le16;
typedef __u16 __be16;
typedef __u32 __be32;
typedef __u64 __be64;
typedef __u32 __wsum;
typedef int pid_t;
typedef struct pid_time_t {
	__u32 pid;
} pid_time_t;

#include <bpf/bpf_helpers.h>

enum bpf_map_type {
	BPF_MAP_TYPE_UNSPEC = 0,
	BPF_MAP_TYPE_HASH = 1,
	BPF_MAP_TYPE_ARRAY = 2,
	BPF_MAP_TYPE_PROG_ARRAY = 3,
	BPF_MAP_TYPE_PERF_EVENT_ARRAY = 4,
	BPF_MAP_TYPE_PERCPU_HASH = 5,
	BPF_MAP_TYPE_PERCPU_ARRAY = 6,
	BPF_MAP_TYPE_STACK_TRACE = 7,
	BPF_MAP_TYPE_CGROUP_ARRAY = 8,
	BPF_MAP_TYPE_LRU_HASH = 9,
	BPF_MAP_TYPE_LRU_PERCPU_HASH = 10,
	BPF_MAP_TYPE_LPM_TRIE = 11,
	BPF_MAP_TYPE_ARRAY_OF_MAPS = 12,
	BPF_MAP_TYPE_HASH_OF_MAPS = 13,
	BPF_MAP_TYPE_DEVMAP = 14,
	BPF_MAP_TYPE_SOCKMAP = 15,
	BPF_MAP_TYPE_CPUMAP = 16,
	BPF_MAP_TYPE_XSKMAP = 17,
	BPF_MAP_TYPE_SOCKHASH = 18,
	BPF_MAP_TYPE_CGROUP_STORAGE = 19,
	BPF_MAP_TYPE_REUSEPORT_SOCKARRAY = 20,
	BPF_MAP_TYPE_PERCPU_CGROUP_STORAGE = 21,
	BPF_MAP_TYPE_QUEUE = 22,
	BPF_MAP_TYPE_STACK = 23,
	BPF_MAP_TYPE_SK_STORAGE = 24,
	BPF_MAP_TYPE_DEVMAP_HASH = 25,
	BPF_MAP_TYPE_STRUCT_OPS = 26,
	BPF_MAP_TYPE_RINGBUF = 27,
	BPF_MAP_TYPE_INODE_STORAGE = 28,
};

enum {
	BPF_ANY = 0,
	BPF_NOEXIST = 1,
	BPF_EXIST = 2,
	BPF_F_LOCK = 4,
};

/* BPF_FUNC_bpf_ringbuf_commit, BPF_FUNC_bpf_ringbuf_discard, and
 * BPF_FUNC_bpf_ringbuf_output flags.
 */
enum {
	BPF_RB_NO_WAKEUP = (1ULL << 0),
	BPF_RB_FORCE_WAKEUP = (1ULL << 1),
};

/* BPF_FUNC_bpf_ringbuf_query flags */
enum {
	BPF_RB_AVAIL_DATA = 0,
	BPF_RB_RING_SIZE = 1,
	BPF_RB_CONS_POS = 2,
	BPF_RB_PROD_POS = 3,
};

enum irq_type {
	NET_TX = 2,
	NET_RX = 3,
	BLOCK = 4
};
const enum irq_type *unused2 __attribute__((unused));

enum {
	BPF_F_INDEX_MASK = 0xffffffffULL,
	BPF_F_CURRENT_CPU = BPF_F_INDEX_MASK,
	/* BPF_FUNC_perf_event_output for sk_buff input context. */
	BPF_F_CTXLEN_MASK = (0xfffffULL << 32),
};

struct bpf_perf_event_value {
	__u64 counter;
	__u64 enabled;
	__u64 running;
};

enum event_type {
	SCHED_SWITCH = 1,
	IRQ = 2,
	PAGE_CACHE_HIT = 3,
	FREE = 4
};

// Force emitting enum event_type into the ELF.
const enum event_type *unused_event_type __attribute__((unused));

struct event {
	u64 event_type;
	u64 ts;
	u32 pid;	      // kernel tgid == userspace pid
	u32 tid;	      // kernel pid == userspace tid
	u32 offcpu_pid;	      // kernel tgid == userspace pid
	u32 offcpu_tid;	      // kernel pid == userspace tid
	u64 offcpu_cgroup_id; // cgroup id is only known for processes going off cpu
	u64 cpu_cycles;
	u64 cpu_instr;
	u64 cache_miss;
	u32 cpu_id;
	u32 irq_number; // one of NET_TX, NET_RX, BLOCK
};

// Force emitting struct event into the ELF.
const struct event *unused_event __attribute__((unused));

struct task_struct {
	int pid;
	unsigned int tgid;
} __attribute__((preserve_access_index));
