// SPDX-License-Identifier: (GPL-2.0-only OR BSD-2-Clause)
// Copyright 2021.

#include "kepler.bpf.h"

SEC("xdp/test/kepler_write_page_trace")
int test_kepler_write_page_trace(void *ctx)
{
	do_page_cache_hit_increment(0);
	return 0;
}

SEC("xdp/test/register_new_process_if_not_exist")
int test_register_new_process_if_not_exist(void *ctx)
{
	register_new_process_if_not_exist(24, 42);
	return 0;
}

char __license[] SEC("license") = "Dual BSD/GPL";
