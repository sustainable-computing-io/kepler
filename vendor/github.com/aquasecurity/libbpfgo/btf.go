package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"fmt"
	"syscall"
)

//
// BTF (low-level API)
//

// GetBTFFDByID returns a file descriptor for the BTF with the given ID.
func GetBTFFDByID(id uint32) (int, error) {
	fdC := C.bpf_btf_get_fd_by_id(C.uint(id))
	if fdC < 0 {
		return int(fdC), fmt.Errorf("could not find BTF id %d: %w", id, syscall.Errno(-fdC))
	}

	return int(fdC), nil
}
