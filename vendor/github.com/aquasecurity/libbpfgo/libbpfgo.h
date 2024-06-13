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
                                                          const char *bpf_obj_name,
                                                          __u32 kernel_log_level);
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

struct bpf_map_info *cgo_bpf_map_info_new();
__u32 cgo_bpf_map_info_size();
void cgo_bpf_map_info_free(struct bpf_map_info *info);

struct bpf_tc_opts *cgo_bpf_tc_opts_new(
    int prog_fd, __u32 flags, __u32 prog_id, __u32 handle, __u32 priority);
void cgo_bpf_tc_opts_free(struct bpf_tc_opts *opts);

struct bpf_tc_hook *cgo_bpf_tc_hook_new();
void cgo_bpf_tc_hook_free(struct bpf_tc_hook *hook);

struct bpf_kprobe_opts *cgo_bpf_kprobe_opts_new(__u64 bpf_cookie,
                                                size_t offset,
                                                bool retprobe,
                                                int attach_mode);
void cgo_bpf_kprobe_opts_free(struct bpf_kprobe_opts *opts);

//
// struct getters
//

// bpf_map_info

__u32 cgo_bpf_map_info_type(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_id(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_key_size(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_value_size(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_max_entries(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_map_flags(struct bpf_map_info *info);
char *cgo_bpf_map_info_name(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_ifindex(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_btf_vmlinux_value_type_id(struct bpf_map_info *info);
__u64 cgo_bpf_map_info_netns_dev(struct bpf_map_info *info);
__u64 cgo_bpf_map_info_netns_ino(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_btf_id(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_btf_key_type_id(struct bpf_map_info *info);
__u32 cgo_bpf_map_info_btf_value_type_id(struct bpf_map_info *info);
__u64 cgo_bpf_map_info_map_extra(struct bpf_map_info *info);

// bpf_tc_opts

int cgo_bpf_tc_opts_prog_fd(struct bpf_tc_opts *opts);
__u32 cgo_bpf_tc_opts_flags(struct bpf_tc_opts *opts);
__u32 cgo_bpf_tc_opts_prog_id(struct bpf_tc_opts *opts);
__u32 cgo_bpf_tc_opts_handle(struct bpf_tc_opts *opts);
__u32 cgo_bpf_tc_opts_priority(struct bpf_tc_opts *opts);

#endif
