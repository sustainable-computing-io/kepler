package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

//
// TcAttachPoint
//

type TcAttachPoint uint32

const (
	BPFTcIngress       TcAttachPoint = C.BPF_TC_INGRESS
	BPFTcEgress        TcAttachPoint = C.BPF_TC_EGRESS
	BPFTcIngressEgress TcAttachPoint = C.BPF_TC_INGRESS | C.BPF_TC_EGRESS
	BPFTcCustom        TcAttachPoint = C.BPF_TC_CUSTOM
)

//
// TcFlags
//

type TcFlags uint32

const (
	BpfTcFReplace TcFlags = C.BPF_TC_F_REPLACE
)
