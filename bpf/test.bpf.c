// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#include "kepler.bpf.h"

SEC("raw_tp")
int test_kepler_write_page_trace(void *ctx)
{
	do_page_cache_hit_increment(0);
	return 0;
}

SEC("raw_tp")
int test_register_new_process_if_not_exist(void *ctx)
{
	register_new_process_if_not_exist(42);
	return 0;
}

SEC("raw_tp/sched_switch")
int test_kepler_sched_switch_trace(u64 *ctx)
{
	do_kepler_sched_switch_trace(42, 43, 42, 43);

	return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
