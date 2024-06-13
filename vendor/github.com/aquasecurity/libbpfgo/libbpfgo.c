#include "libbpfgo.h"

extern void loggerCallback(enum libbpf_print_level level, char *output);
extern void perfCallback(void *ctx, int cpu, void *data, __u32 size);
extern void perfLostCallback(void *ctx, int cpu, __u64 cnt);
extern int ringbufferCallback(void *ctx, void *data, size_t size);

int libbpf_print_fn(enum libbpf_print_level level, // libbpf print level
                    const char *format,            // format used for the msg
                    va_list args)                  // args used by format
{
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

void cgo_libbpf_set_print_fn()
{
    libbpf_set_print(libbpf_print_fn);
}

struct ring_buffer *cgo_init_ring_buf(int map_fd, uintptr_t ctx)
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

struct perf_buffer *cgo_init_perf_buf(int map_fd, int page_cnt, uintptr_t ctx)
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

void cgo_bpf_map__initial_value(struct bpf_map *map, void *value)
{
    size_t psize;
    const void *data;

    data = bpf_map__initial_value(map, &psize);
    if (!data)
        return;

    memcpy(value, data, psize);
}

int cgo_bpf_prog_attach_cgroup_legacy(int prog_fd,   // eBPF program file descriptor
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

int cgo_bpf_prog_detach_cgroup_legacy(int prog_fd,   // eBPF program file descriptor
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

//
// struct handlers
//

struct bpf_iter_attach_opts *cgo_bpf_iter_attach_opts_new(__u32 map_fd,
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

void cgo_bpf_iter_attach_opts_free(struct bpf_iter_attach_opts *opts)
{
    if (!opts)
        return;

    free(opts->link_info);
    free(opts);
}

struct bpf_object_open_opts *cgo_bpf_object_open_opts_new(const char *btf_file_path,
                                                          const char *kconfig_path,
                                                          const char *bpf_obj_name,
                                                          __u32 kernel_log_level)
{
    struct bpf_object_open_opts *opts;
    opts = calloc(1, sizeof(*opts));
    if (!opts)
        return NULL;

    opts->sz = sizeof(*opts);
    opts->btf_custom_path = btf_file_path;
    opts->kconfig = kconfig_path;
    opts->object_name = bpf_obj_name;
    opts->kernel_log_level = kernel_log_level;

    return opts;
}

void cgo_bpf_object_open_opts_free(struct bpf_object_open_opts *opts)
{
    free(opts);
}

struct bpf_map_create_opts *cgo_bpf_map_create_opts_new(__u32 btf_fd,
                                                        __u32 btf_key_type_id,
                                                        __u32 btf_value_type_id,
                                                        __u32 btf_vmlinux_value_type_id,
                                                        __u32 inner_map_fd,
                                                        __u32 map_flags,
                                                        __u64 map_extra,
                                                        __u32 numa_node,
                                                        __u32 map_ifindex)
{
    struct bpf_map_create_opts *opts;
    opts = calloc(1, sizeof(*opts));
    if (!opts)
        return NULL;

    opts->sz = sizeof(*opts);
    opts->btf_fd = btf_fd;
    opts->btf_key_type_id = btf_key_type_id;
    opts->btf_value_type_id = btf_value_type_id;
    opts->btf_vmlinux_value_type_id = btf_vmlinux_value_type_id;
    opts->inner_map_fd = inner_map_fd;
    opts->map_flags = map_flags;
    opts->map_extra = map_extra;
    opts->numa_node = numa_node;
    opts->map_ifindex = map_ifindex;

    return opts;
}

void cgo_bpf_map_create_opts_free(struct bpf_map_create_opts *opts)
{
    free(opts);
}

struct bpf_map_batch_opts *cgo_bpf_map_batch_opts_new(__u64 elem_flags, __u64 flags)
{
    struct bpf_map_batch_opts *opts;
    opts = calloc(1, sizeof(*opts));
    if (!opts)
        return NULL;

    opts->sz = sizeof(*opts);
    opts->elem_flags = elem_flags;
    opts->flags = flags;

    return opts;
}

void cgo_bpf_map_batch_opts_free(struct bpf_map_batch_opts *opts)
{
    free(opts);
}

struct bpf_map_info *cgo_bpf_map_info_new()
{
    struct bpf_map_info *info;
    info = calloc(1, sizeof(*info));
    if (!info)
        return NULL;

    return info;
}

__u32 cgo_bpf_map_info_size()
{
    return sizeof(struct bpf_map_info);
}

void cgo_bpf_map_info_free(struct bpf_map_info *info)
{
    free(info);
}

struct bpf_tc_opts *cgo_bpf_tc_opts_new(
    int prog_fd, __u32 flags, __u32 prog_id, __u32 handle, __u32 priority)
{
    struct bpf_tc_opts *opts;
    opts = calloc(1, sizeof(*opts));
    if (!opts)
        return NULL;

    opts->sz = sizeof(*opts);
    opts->prog_fd = prog_fd;
    opts->flags = flags;
    opts->prog_id = prog_id;
    opts->handle = handle;
    opts->priority = priority;

    return opts;
}

void cgo_bpf_tc_opts_free(struct bpf_tc_opts *opts)
{
    free(opts);
}

struct bpf_tc_hook *cgo_bpf_tc_hook_new()
{
    struct bpf_tc_hook *hook;
    hook = calloc(1, sizeof(*hook));
    if (!hook)
        return NULL;

    hook->sz = sizeof(*hook);

    return hook;
}

void cgo_bpf_tc_hook_free(struct bpf_tc_hook *hook)
{
    free(hook);
}

struct bpf_kprobe_opts *cgo_bpf_kprobe_opts_new(__u64 bpf_cookie,
                                                size_t offset,
                                                bool retprobe,
                                                int attach_mode)
{
    struct bpf_kprobe_opts *opts;
    opts = calloc(1, sizeof(*opts));
    if (!opts)
        return NULL;

    opts->sz = sizeof(*opts);
    opts->bpf_cookie = bpf_cookie;
    opts->offset = offset;
    opts->retprobe = retprobe;
    opts->attach_mode = attach_mode;

    return opts;
}

void cgo_bpf_kprobe_opts_free(struct bpf_kprobe_opts *opts)
{
    free(opts);
}

//
// struct getters
//

// bpf_map_info

__u32 cgo_bpf_map_info_type(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->type;
}

__u32 cgo_bpf_map_info_id(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->id;
}

__u32 cgo_bpf_map_info_key_size(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->key_size;
}

__u32 cgo_bpf_map_info_value_size(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->value_size;
}

__u32 cgo_bpf_map_info_max_entries(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->max_entries;
}

__u32 cgo_bpf_map_info_map_flags(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->map_flags;
}

char *cgo_bpf_map_info_name(struct bpf_map_info *info)
{
    if (!info)
        return NULL;

    return info->name;
}

__u32 cgo_bpf_map_info_ifindex(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->ifindex;
}

__u32 cgo_bpf_map_info_btf_vmlinux_value_type_id(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->btf_vmlinux_value_type_id;
}

__u64 cgo_bpf_map_info_netns_dev(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->netns_dev;
}

__u64 cgo_bpf_map_info_netns_ino(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->netns_ino;
}

__u32 cgo_bpf_map_info_btf_id(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->btf_id;
}

__u32 cgo_bpf_map_info_btf_key_type_id(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->btf_key_type_id;
}

__u32 cgo_bpf_map_info_btf_value_type_id(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->btf_value_type_id;
}

__u64 cgo_bpf_map_info_map_extra(struct bpf_map_info *info)
{
    if (!info)
        return 0;

    return info->map_extra;
}

// bpf_tc_opts

int cgo_bpf_tc_opts_prog_fd(struct bpf_tc_opts *opts)
{
    if (!opts)
        return 0;

    return opts->prog_fd;
}

__u32 cgo_bpf_tc_opts_flags(struct bpf_tc_opts *opts)
{
    if (!opts)
        return 0;

    return opts->flags;
}

__u32 cgo_bpf_tc_opts_prog_id(struct bpf_tc_opts *opts)
{
    if (!opts)
        return 0;

    return opts->prog_id;
}

__u32 cgo_bpf_tc_opts_handle(struct bpf_tc_opts *opts)
{
    if (!opts)
        return 0;

    return opts->handle;
}

__u32 cgo_bpf_tc_opts_priority(struct bpf_tc_opts *opts)
{
    if (!opts)
        return 0;

    return opts->priority;
}
