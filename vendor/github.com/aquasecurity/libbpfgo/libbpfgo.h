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

extern void loggerCallback(enum libbpf_print_level level, char *output);

int libbpf_print_fn(enum libbpf_print_level level, // libbpf print level
                    const char *format,            // format used for the msg
                    va_list args)
{ // args used by format

    int ret;
    size_t len;
    char *out;
    va_list check;

    va_copy(check, args);
    ret = vsnprintf(NULL, 0, format, check); // get output length
    va_end(check);

    if (ret < 0)
        return ret;

    len = ret + 1; // add 1 for NUL
    out = malloc(len);
    if (!out)
        return -ENOMEM;

    va_copy(check, args);
    ret = vsnprintf(out, len, format, check);
    va_end(check);

    if (ret > 0)
        loggerCallback(level, out);

    free(out);

    return ret;
}

void set_print_fn()
{
    libbpf_set_print(libbpf_print_fn);
}

extern void perfCallback(void *ctx, int cpu, void *data, __u32 size);
extern void perfLostCallback(void *ctx, int cpu, __u64 cnt);
extern int ringbufferCallback(void *ctx, void *data, size_t size);

struct ring_buffer *init_ring_buf(int map_fd, uintptr_t ctx)
{
    struct ring_buffer *rb = NULL;

    rb = ring_buffer__new(map_fd, ringbufferCallback, (void *) ctx, NULL);
    if (!rb) {
        int saved_errno = errno;
        fprintf(stderr, "Failed to initialize ring buffer: %s\n", strerror(errno));
        errno = saved_errno;
        return NULL;
    }

    return rb;
}

struct perf_buffer *init_perf_buf(int map_fd, int page_cnt, uintptr_t ctx)
{
    struct perf_buffer_opts pb_opts = {};
    struct perf_buffer *pb = NULL;

    pb_opts.sz = sizeof(struct perf_buffer_opts);

    pb = perf_buffer__new(map_fd, page_cnt, perfCallback, perfLostCallback, (void *) ctx, &pb_opts);
    if (!pb) {
        int saved_errno = errno;
        fprintf(stderr, "Failed to initialize perf buffer: %s\n", strerror(errno));
        errno = saved_errno;
        return NULL;
    }

    return pb;
}

void get_internal_map_init_value(struct bpf_map *map, void *value)
{
    size_t psize;
    const void *data;
    data = bpf_map__initial_value(map, &psize);
    memcpy(value, data, psize);
}

int bpf_prog_attach_cgroup_legacy(int prog_fd,   // eBPF program file descriptor
                                  int target_fd, // cgroup directory file descriptor
                                  int type)      // BPF_CGROUP_INET_{INGRESS,EGRESS}, ...
{
    union bpf_attr attr;
    memset(&attr, 0, sizeof(attr));
    attr.target_fd = target_fd;
    attr.attach_bpf_fd = prog_fd;
    attr.attach_type = type;
    attr.attach_flags = BPF_F_ALLOW_MULTI; // or BPF_F_ALLOW_OVERRIDE

    return syscall(__NR_bpf, BPF_PROG_ATTACH, &attr, sizeof(attr));
}

int bpf_prog_detach_cgroup_legacy(int prog_fd,   // eBPF program file descriptor
                                  int target_fd, // cgroup directory file descriptor
                                  int type)      // BPF_CGROUP_INET_{INGRESS,EGRESS}, ...
{
    union bpf_attr attr;
    memset(&attr, 0, sizeof(attr));
    attr.target_fd = target_fd;
    attr.attach_bpf_fd = prog_fd;
    attr.attach_type = type;

    return syscall(__NR_bpf, BPF_PROG_DETACH, &attr, sizeof(attr));
}

struct bpf_iter_attach_opts *bpf_iter_attach_opts_new(__u32 map_fd,
                                                      enum bpf_cgroup_iter_order order,
                                                      __u32 cgroup_fd,
                                                      __u64 cgroup_id,
                                                      __u32 tid,
                                                      __u32 pid,
                                                      __u32 pid_fd)
{
    union bpf_iter_link_info *linfo;
    linfo = calloc(1, sizeof(*linfo));
    if (!linfo)
        return NULL;

    linfo->map.map_fd = map_fd;
    linfo->cgroup.order = order;
    linfo->cgroup.cgroup_fd = cgroup_fd;
    linfo->cgroup.cgroup_id = cgroup_id;
    linfo->task.tid = tid;
    linfo->task.pid = pid;
    linfo->task.pid_fd = pid_fd;

    struct bpf_iter_attach_opts *opts;
    opts = calloc(1, sizeof(*opts));
    if (!opts) {
        free(linfo);
        return NULL;
    }

    opts->sz = sizeof(*opts);
    opts->link_info_len = sizeof(*linfo);
    opts->link_info = linfo;

    return opts;
}

void bpf_iter_attach_opts_free(struct bpf_iter_attach_opts *opts)
{
    if (!opts)
        return;

    free(opts->link_info);
    free(opts);
}

struct bpf_object_open_opts *
bpf_object_open_opts_new(char *btf_file_path, char *kconfig_path, char *bpf_obj_name)
{
    struct bpf_object_open_opts *opts;
    opts = calloc(1, sizeof(*opts));
    if (!opts)
        return NULL;

    opts->sz = sizeof(*opts);
    opts->btf_custom_path = btf_file_path;
    opts->kconfig = kconfig_path;
    opts->object_name = bpf_obj_name;

    return opts;
}

void bpf_object_open_opts_free(struct bpf_object_open_opts *opts)
{
    free(opts);
}

#endif
