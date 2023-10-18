#ifndef __LIBBPF_GO_H__
#define __LIBBPF_GO_H__

#ifdef __powerpc64__
    #define __SANE_USERSPACE_TYPES__ 1
#endif

#include <stdio.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>
#include <sys/resource.h>
#include <sys/syscall.h>
#include <unistd.h>

#include <bpf/bpf.h>
#include <bpf/libbpf.h>
#include <linux/bpf.h> // uapi

void cgo_libbpf_set_print_fn();

struct ring_buffer *cgo_init_ring_buf(int map_fd, uintptr_t ctx);
struct perf_buffer *cgo_init_perf_buf(int map_fd, int page_cnt, uintptr_t ctx);

void cgo_bpf_map__initial_value(struct bpf_map *map, void *value);

int cgo_bpf_prog_attach_cgroup_legacy(int prog_fd, int target_fd, int type);
int cgo_bpf_prog_detach_cgroup_legacy(int prog_fd, int target_fd, int type);

//
// struct handlers
//

struct bpf_iter_attach_opts *cgo_bpf_iter_attach_opts_new(__u32 map_fd,
                                                          enum bpf_cgroup_iter_order order,
                                                          __u32 cgroup_fd,
                                                          __u64 cgroup_id,
                                                          __u32 tid,
                                                          __u32 pid,
                                                          __u32 pid_fd);
void cgo_bpf_iter_attach_opts_free(struct bpf_iter_attach_opts *opts);

struct bpf_object_open_opts *cgo_bpf_object_open_opts_new(const char *btf_file_path,
                                                          const char *kconfig_path,
                                                          const char *bpf_obj_name);
void cgo_bpf_object_open_opts_free(struct bpf_object_open_opts *opts);

struct bpf_map_create_opts *cgo_bpf_map_create_opts_new(__u32 btf_fd,
                                                        __u32 btf_key_type_id,
                                                        __u32 btf_value_type_id,
                                                        __u32 btf_vmlinux_value_type_id,
                                                        __u32 inner_map_fd,
                                                        __u32 map_flags,
                                                        __u64 map_extra,
                                                        __u32 numa_node,
                                                        __u32 map_ifindex);
void cgo_bpf_map_create_opts_free(struct bpf_map_create_opts *opts);

struct bpf_map_batch_opts *cgo_bpf_map_batch_opts_new(__u64 elem_flags, __u64 flags);
void cgo_bpf_map_batch_opts_free(struct bpf_map_batch_opts *opts);

#endif
