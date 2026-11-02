// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux && s390x

package device

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl request numbers from arch/s390/include/uapi/asm/diag.h
//
//	DIAG324_GET_PIBBUF = _IOWR('D', 0x77, struct diag324_pib)  // 16 bytes
//	DIAG324_GET_PIBLEN = _IOR('D', 0x78, size_t)               // 8 bytes
const (
	diag324GetPIBBuf = 0xC0104477
	diag324GetPIBLen = 0x80084478
)

// diag324PIB mirrors struct diag324_pib.
type diag324PIB struct {
	Address  uint64 // user-space address of the PIB buffer
	Sequence uint64 // filled in by the kernel
}

// diagDevice reads PIBs from /dev/diag.
type diagDevice struct {
	fd  int
	buf []byte // anonymous mmap so the GC never moves or frees it under the kernel
}

func openPIBSource(path string) (pibSource, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, fmt.Errorf("%w: %s not found (needs a kernel with DIAG 0x324 support, i.e. Linux >= 6.14 or a distro backport, and access to host /dev)", errPIBNotSupported, path)
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	var size uintptr // size_t
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), diag324GetPIBLen, uintptr(unsafe.Pointer(&size))); errno != 0 {
		_ = unix.Close(fd)
		if errno == unix.EOPNOTSUPP || errno == unix.ENOTTY {
			return nil, fmt.Errorf("%w: DIAG324_GET_PIBLEN: %v", errPIBNotSupported, errno)
		}
		return nil, fmt.Errorf("DIAG324_GET_PIBLEN: %w", errno)
	}
	if size < pibHeaderMinLen {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("%w: kernel reported PIB length %d", errPIBNotSupported, size)
	}

	pageSize := uintptr(os.Getpagesize())
	mapLen := int((size + pageSize - 1) &^ (pageSize - 1))
	buf, err := unix.Mmap(-1, 0, mapLen, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("allocate PIB buffer: %w", err)
	}

	return &diagDevice{fd: fd, buf: buf[:size]}, nil
}

func (d *diagDevice) ReadPIB() ([]byte, uint64, error) {
	clear(d.buf)
	req := diag324PIB{Address: uint64(uintptr(unsafe.Pointer(&d.buf[0])))}

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(d.fd), diag324GetPIBBuf, uintptr(unsafe.Pointer(&req)))
	runtime.KeepAlive(&req)

	out := make([]byte, len(d.buf))
	copy(out, d.buf)

	switch errno {
	case 0:
		return out, req.Sequence, nil
	case unix.EBUSY:
		// The kernel still copies the (stale) PIB on EBUSY.
		return out, req.Sequence, errPIBBusy
	case unix.EOPNOTSUPP:
		return nil, 0, fmt.Errorf("%w: DIAG324_GET_PIBBUF: %v", errPIBNotSupported, errno)
	default:
		return nil, 0, fmt.Errorf("DIAG324_GET_PIBBUF: %w", errno)
	}
}

func (d *diagDevice) Close() error {
	var errs []error
	if d.buf != nil {
		errs = append(errs, unix.Munmap(d.buf[:cap(d.buf)]))
		d.buf = nil
	}
	if d.fd >= 0 {
		errs = append(errs, unix.Close(d.fd))
		d.fd = -1
	}
	return errors.Join(errs...)
}
