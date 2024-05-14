package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

//
// BPFProgType
//

// BPFProgType is an enum as defined in https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/bpf.h
type BPFProgType uint32

const (
	BPFProgTypeUnspec                BPFProgType = C.BPF_PROG_TYPE_UNSPEC
	BPFProgTypeSocketFilter          BPFProgType = C.BPF_PROG_TYPE_SOCKET_FILTER
	BPFProgTypeKprobe                BPFProgType = C.BPF_PROG_TYPE_KPROBE
	BPFProgTypeSchedCls              BPFProgType = C.BPF_PROG_TYPE_SCHED_CLS
	BPFProgTypeSchedAct              BPFProgType = C.BPF_PROG_TYPE_SCHED_ACT
	BPFProgTypeTracepoint            BPFProgType = C.BPF_PROG_TYPE_TRACEPOINT
	BPFProgTypeXdp                   BPFProgType = C.BPF_PROG_TYPE_XDP
	BPFProgTypePerfEvent             BPFProgType = C.BPF_PROG_TYPE_PERF_EVENT
	BPFProgTypeCgroupSkb             BPFProgType = C.BPF_PROG_TYPE_CGROUP_SKB
	BPFProgTypeCgroupSock            BPFProgType = C.BPF_PROG_TYPE_CGROUP_SOCK
	BPFProgTypeLwtIn                 BPFProgType = C.BPF_PROG_TYPE_LWT_IN
	BPFProgTypeLwtOut                BPFProgType = C.BPF_PROG_TYPE_LWT_OUT
	BPFProgTypeLwtXmit               BPFProgType = C.BPF_PROG_TYPE_LWT_XMIT
	BPFProgTypeSockOps               BPFProgType = C.BPF_PROG_TYPE_SOCK_OPS
	BPFProgTypeSkSkb                 BPFProgType = C.BPF_PROG_TYPE_SK_SKB
	BPFProgTypeCgroupDevice          BPFProgType = C.BPF_PROG_TYPE_CGROUP_DEVICE
	BPFProgTypeSkMsg                 BPFProgType = C.BPF_PROG_TYPE_SK_MSG
	BPFProgTypeRawTracepoint         BPFProgType = C.BPF_PROG_TYPE_RAW_TRACEPOINT
	BPFProgTypeCgroupSockAddr        BPFProgType = C.BPF_PROG_TYPE_CGROUP_SOCK_ADDR
	BPFProgTypeLwtSeg6Local          BPFProgType = C.BPF_PROG_TYPE_LWT_SEG6LOCAL
	BPFProgTypeLircMode2             BPFProgType = C.BPF_PROG_TYPE_LIRC_MODE2
	BPFProgTypeSkReuseport           BPFProgType = C.BPF_PROG_TYPE_SK_REUSEPORT
	BPFProgTypeFlowDissector         BPFProgType = C.BPF_PROG_TYPE_FLOW_DISSECTOR
	BPFProgTypeCgroupSysctl          BPFProgType = C.BPF_PROG_TYPE_CGROUP_SYSCTL
	BPFProgTypeRawTracepointWritable BPFProgType = C.BPF_PROG_TYPE_RAW_TRACEPOINT_WRITABLE
	BPFProgTypeCgroupSockopt         BPFProgType = C.BPF_PROG_TYPE_CGROUP_SOCKOPT
	BPFProgTypeTracing               BPFProgType = C.BPF_PROG_TYPE_TRACING
	BPFProgTypeStructOps             BPFProgType = C.BPF_PROG_TYPE_STRUCT_OPS
	BPFProgTypeExt                   BPFProgType = C.BPF_PROG_TYPE_EXT
	BPFProgTypeLsm                   BPFProgType = C.BPF_PROG_TYPE_LSM
	BPFProgTypeSkLookup              BPFProgType = C.BPF_PROG_TYPE_SK_LOOKUP
	BPFProgTypeSyscall               BPFProgType = C.BPF_PROG_TYPE_SYSCALL
)

// Deprecated: Convert type directly instead.
func (t BPFProgType) Value() uint64 { return uint64(t) }

var bpfProgTypeToString = map[BPFProgType]string{
	BPFProgTypeUnspec:                "BPF_PROG_TYPE_UNSPEC",
	BPFProgTypeSocketFilter:          "BPF_PROG_TYPE_SOCKET_FILTER",
	BPFProgTypeKprobe:                "BPF_PROG_TYPE_KPROBE",
	BPFProgTypeSchedCls:              "BPF_PROG_TYPE_SCHED_CLS",
	BPFProgTypeSchedAct:              "BPF_PROG_TYPE_SCHED_ACT",
	BPFProgTypeTracepoint:            "BPF_PROG_TYPE_TRACEPOINT",
	BPFProgTypeXdp:                   "BPF_PROG_TYPE_XDP",
	BPFProgTypePerfEvent:             "BPF_PROG_TYPE_PERF_EVENT",
	BPFProgTypeCgroupSkb:             "BPF_PROG_TYPE_CGROUP_SKB",
	BPFProgTypeCgroupSock:            "BPF_PROG_TYPE_CGROUP_SOCK",
	BPFProgTypeLwtIn:                 "BPF_PROG_TYPE_LWT_IN",
	BPFProgTypeLwtOut:                "BPF_PROG_TYPE_LWT_OUT",
	BPFProgTypeLwtXmit:               "BPF_PROG_TYPE_LWT_XMIT",
	BPFProgTypeSockOps:               "BPF_PROG_TYPE_SOCK_OPS",
	BPFProgTypeSkSkb:                 "BPF_PROG_TYPE_SK_SKB",
	BPFProgTypeCgroupDevice:          "BPF_PROG_TYPE_CGROUP_DEVICE",
	BPFProgTypeSkMsg:                 "BPF_PROG_TYPE_SK_MSG",
	BPFProgTypeRawTracepoint:         "BPF_PROG_TYPE_RAW_TRACEPOINT",
	BPFProgTypeCgroupSockAddr:        "BPF_PROG_TYPE_CGROUP_SOCK_ADDR",
	BPFProgTypeLwtSeg6Local:          "BPF_PROG_TYPE_LWT_SEG6LOCAL",
	BPFProgTypeLircMode2:             "BPF_PROG_TYPE_LIRC_MODE2",
	BPFProgTypeSkReuseport:           "BPF_PROG_TYPE_SK_REUSEPORT",
	BPFProgTypeFlowDissector:         "BPF_PROG_TYPE_FLOW_DISSECTOR",
	BPFProgTypeCgroupSysctl:          "BPF_PROG_TYPE_CGROUP_SYSCTL",
	BPFProgTypeRawTracepointWritable: "BPF_PROG_TYPE_RAW_TRACEPOINT_WRITABLE",
	BPFProgTypeCgroupSockopt:         "BPF_PROG_TYPE_CGROUP_SOCKOPT",
	BPFProgTypeTracing:               "BPF_PROG_TYPE_TRACING",
	BPFProgTypeStructOps:             "BPF_PROG_TYPE_STRUCT_OPS",
	BPFProgTypeExt:                   "BPF_PROG_TYPE_EXT",
	BPFProgTypeLsm:                   "BPF_PROG_TYPE_LSM",
	BPFProgTypeSkLookup:              "BPF_PROG_TYPE_SK_LOOKUP",
	BPFProgTypeSyscall:               "BPF_PROG_TYPE_SYSCALL",
}

func (t BPFProgType) String() string {
	str, ok := bpfProgTypeToString[t]
	if !ok {
		// BPFProgTypeUnspec must exist in bpfProgTypeToString to avoid infinite recursion.
		return BPFProgTypeUnspec.String()
	}

	return str
}

func (t BPFProgType) Name() string {
	return C.GoString(C.libbpf_bpf_prog_type_str(C.enum_bpf_prog_type(t)))
}

//
// BPFAttachType
//

type BPFAttachType uint32

const (
	BPFAttachTypeCgroupInetIngress          BPFAttachType = C.BPF_CGROUP_INET_INGRESS
	BPFAttachTypeCgroupInetEgress           BPFAttachType = C.BPF_CGROUP_INET_EGRESS
	BPFAttachTypeCgroupInetSockCreate       BPFAttachType = C.BPF_CGROUP_INET_SOCK_CREATE
	BPFAttachTypeCgroupSockOps              BPFAttachType = C.BPF_CGROUP_SOCK_OPS
	BPFAttachTypeSKSKBStreamParser          BPFAttachType = C.BPF_SK_SKB_STREAM_PARSER
	BPFAttachTypeSKSKBStreamVerdict         BPFAttachType = C.BPF_SK_SKB_STREAM_VERDICT
	BPFAttachTypeCgroupDevice               BPFAttachType = C.BPF_CGROUP_DEVICE
	BPFAttachTypeSKMSGVerdict               BPFAttachType = C.BPF_SK_MSG_VERDICT
	BPFAttachTypeCgroupInet4Bind            BPFAttachType = C.BPF_CGROUP_INET4_BIND
	BPFAttachTypeCgroupInet6Bind            BPFAttachType = C.BPF_CGROUP_INET6_BIND
	BPFAttachTypeCgroupInet4Connect         BPFAttachType = C.BPF_CGROUP_INET4_CONNECT
	BPFAttachTypeCgroupInet6Connect         BPFAttachType = C.BPF_CGROUP_INET6_CONNECT
	BPFAttachTypeCgroupInet4PostBind        BPFAttachType = C.BPF_CGROUP_INET4_POST_BIND
	BPFAttachTypeCgroupInet6PostBind        BPFAttachType = C.BPF_CGROUP_INET6_POST_BIND
	BPFAttachTypeCgroupUDP4SendMsg          BPFAttachType = C.BPF_CGROUP_UDP4_SENDMSG
	BPFAttachTypeCgroupUDP6SendMsg          BPFAttachType = C.BPF_CGROUP_UDP6_SENDMSG
	BPFAttachTypeLircMode2                  BPFAttachType = C.BPF_LIRC_MODE2
	BPFAttachTypeFlowDissector              BPFAttachType = C.BPF_FLOW_DISSECTOR
	BPFAttachTypeCgroupSysctl               BPFAttachType = C.BPF_CGROUP_SYSCTL
	BPFAttachTypeCgroupUDP4RecvMsg          BPFAttachType = C.BPF_CGROUP_UDP4_RECVMSG
	BPFAttachTypeCgroupUDP6RecvMsg          BPFAttachType = C.BPF_CGROUP_UDP6_RECVMSG
	BPFAttachTypeCgroupGetSockOpt           BPFAttachType = C.BPF_CGROUP_GETSOCKOPT
	BPFAttachTypeCgroupSetSockOpt           BPFAttachType = C.BPF_CGROUP_SETSOCKOPT
	BPFAttachTypeTraceRawTP                 BPFAttachType = C.BPF_TRACE_RAW_TP
	BPFAttachTypeTraceFentry                BPFAttachType = C.BPF_TRACE_FENTRY
	BPFAttachTypeTraceFexit                 BPFAttachType = C.BPF_TRACE_FEXIT
	BPFAttachTypeModifyReturn               BPFAttachType = C.BPF_MODIFY_RETURN
	BPFAttachTypeLSMMac                     BPFAttachType = C.BPF_LSM_MAC
	BPFAttachTypeTraceIter                  BPFAttachType = C.BPF_TRACE_ITER
	BPFAttachTypeCgroupInet4GetPeerName     BPFAttachType = C.BPF_CGROUP_INET4_GETPEERNAME
	BPFAttachTypeCgroupInet6GetPeerName     BPFAttachType = C.BPF_CGROUP_INET6_GETPEERNAME
	BPFAttachTypeCgroupInet4GetSockName     BPFAttachType = C.BPF_CGROUP_INET4_GETSOCKNAME
	BPFAttachTypeCgroupInet6GetSockName     BPFAttachType = C.BPF_CGROUP_INET6_GETSOCKNAME
	BPFAttachTypeXDPDevMap                  BPFAttachType = C.BPF_XDP_DEVMAP
	BPFAttachTypeCgroupInetSockRelease      BPFAttachType = C.BPF_CGROUP_INET_SOCK_RELEASE
	BPFAttachTypeXDPCPUMap                  BPFAttachType = C.BPF_XDP_CPUMAP
	BPFAttachTypeSKLookup                   BPFAttachType = C.BPF_SK_LOOKUP
	BPFAttachTypeXDP                        BPFAttachType = C.BPF_XDP
	BPFAttachTypeSKSKBVerdict               BPFAttachType = C.BPF_SK_SKB_VERDICT
	BPFAttachTypeSKReusePortSelect          BPFAttachType = C.BPF_SK_REUSEPORT_SELECT
	BPFAttachTypeSKReusePortSelectorMigrate BPFAttachType = C.BPF_SK_REUSEPORT_SELECT_OR_MIGRATE
	BPFAttachTypePerfEvent                  BPFAttachType = C.BPF_PERF_EVENT
	BPFAttachTypeTraceKprobeMulti           BPFAttachType = C.BPF_TRACE_KPROBE_MULTI
)

var bpfAttachTypeToString = map[BPFAttachType]string{
	BPFAttachTypeCgroupInetIngress:          "BPF_CGROUP_INET_INGRESS",
	BPFAttachTypeCgroupInetEgress:           "BPF_CGROUP_INET_EGRESS",
	BPFAttachTypeCgroupInetSockCreate:       "BPF_CGROUP_INET_SOCK_CREATE",
	BPFAttachTypeCgroupSockOps:              "BPF_CGROUP_SOCK_OPS",
	BPFAttachTypeSKSKBStreamParser:          "BPF_SK_SKB_STREAM_PARSER",
	BPFAttachTypeSKSKBStreamVerdict:         "BPF_SK_SKB_STREAM_VERDICT",
	BPFAttachTypeCgroupDevice:               "BPF_CGROUP_DEVICE",
	BPFAttachTypeSKMSGVerdict:               "BPF_SK_MSG_VERDICT",
	BPFAttachTypeCgroupInet4Bind:            "BPF_CGROUP_INET4_BIND",
	BPFAttachTypeCgroupInet6Bind:            "BPF_CGROUP_INET6_BIND",
	BPFAttachTypeCgroupInet4Connect:         "BPF_CGROUP_INET4_CONNECT",
	BPFAttachTypeCgroupInet6Connect:         "BPF_CGROUP_INET6_CONNECT",
	BPFAttachTypeCgroupInet4PostBind:        "BPF_CGROUP_INET4_POST_BIND",
	BPFAttachTypeCgroupInet6PostBind:        "BPF_CGROUP_INET6_POST_BIND",
	BPFAttachTypeCgroupUDP4SendMsg:          "BPF_CGROUP_UDP4_SENDMSG",
	BPFAttachTypeCgroupUDP6SendMsg:          "BPF_CGROUP_UDP6_SENDMSG",
	BPFAttachTypeLircMode2:                  "BPF_LIRC_MODE2",
	BPFAttachTypeFlowDissector:              "BPF_FLOW_DISSECTOR",
	BPFAttachTypeCgroupSysctl:               "BPF_CGROUP_SYSCTL",
	BPFAttachTypeCgroupUDP4RecvMsg:          "BPF_CGROUP_UDP4_RECVMSG",
	BPFAttachTypeCgroupUDP6RecvMsg:          "BPF_CGROUP_UDP6_RECVMSG",
	BPFAttachTypeCgroupGetSockOpt:           "BPF_CGROUP_GETSOCKOPT",
	BPFAttachTypeCgroupSetSockOpt:           "BPF_CGROUP_SETSOCKOPT",
	BPFAttachTypeTraceRawTP:                 "BPF_TRACE_RAW_TP",
	BPFAttachTypeTraceFentry:                "BPF_TRACE_FENTRY",
	BPFAttachTypeTraceFexit:                 "BPF_TRACE_FEXIT",
	BPFAttachTypeModifyReturn:               "BPF_MODIFY_RETURN",
	BPFAttachTypeLSMMac:                     "BPF_LSM_MAC",
	BPFAttachTypeTraceIter:                  "BPF_TRACE_ITER",
	BPFAttachTypeCgroupInet4GetPeerName:     "BPF_CGROUP_INET4_GETPEERNAME",
	BPFAttachTypeCgroupInet6GetPeerName:     "BPF_CGROUP_INET6_GETPEERNAME",
	BPFAttachTypeCgroupInet4GetSockName:     "BPF_CGROUP_INET4_GETSOCKNAME",
	BPFAttachTypeCgroupInet6GetSockName:     "BPF_CGROUP_INET6_GETSOCKNAME",
	BPFAttachTypeXDPDevMap:                  "BPF_XDP_DEVMAP",
	BPFAttachTypeCgroupInetSockRelease:      "BPF_CGROUP_INET_SOCK_RELEASE",
	BPFAttachTypeXDPCPUMap:                  "BPF_XDP_CPUMAP",
	BPFAttachTypeSKLookup:                   "BPF_SK_LOOKUP",
	BPFAttachTypeXDP:                        "BPF_XDP",
	BPFAttachTypeSKSKBVerdict:               "BPF_SK_SKB_VERDICT",
	BPFAttachTypeSKReusePortSelect:          "BPF_SK_REUSEPORT_SELECT",
	BPFAttachTypeSKReusePortSelectorMigrate: "BPF_SK_REUSEPORT_SELECT_OR_MIGRATE",
	BPFAttachTypePerfEvent:                  "BPF_PERF_EVENT",
	BPFAttachTypeTraceKprobeMulti:           "BPF_TRACE_KPROBE_MULTI",
}

func (t BPFAttachType) String() string {
	str, ok := bpfAttachTypeToString[t]
	if !ok {
		return "BPFAttachType unspecified"
	}

	return str
}

func (t BPFAttachType) Name() string {
	return C.GoString(C.libbpf_bpf_attach_type_str(C.enum_bpf_attach_type(t)))
}

//
// BPFCgroupIterOrder
//

type BPFCgroupIterOrder uint32

const (
	BPFIterOrderUnspec BPFCgroupIterOrder = iota
	BPFIterSelfOnly
	BPFIterDescendantsPre
	BPFIterDescendantsPost
	BPFIterAncestorsUp
)

//
// AttachFlag
//

type AttachFlag uint32

const (
	BPFFNone          AttachFlag = 0
	BPFFAllowOverride AttachFlag = C.BPF_F_ALLOW_OVERRIDE
	BPFFAllowMulti    AttachFlag = C.BPF_F_ALLOW_MULTI
	BPFFReplace       AttachFlag = C.BPF_F_REPLACE
)
