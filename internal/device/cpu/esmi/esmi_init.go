// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

//go:build goamdsmi

package esmi

/*
#cgo CFLAGS: -I/opt/e-sms/e_smi/include
#cgo LDFLAGS: -L/opt/e-sms/e_smi/lib -le_smi64

#include <stdint.h>
#include <e_smi/e_smi.h>

static int esmi_init_wrapper(void) {
    return (int)esmi_init();
}
*/
import "C"
import "sync"

// Shared E-SMI library initialization state.
// The E-SMI library must be initialized only once per process.
var (
	esmiInitOnce sync.Once
	esmiInitOk   bool
)

// esmiInit ensures the E-SMI library is initialized exactly once.
// Returns true if initialization succeeded (or was already successful).
func esmiInit() bool {
	esmiInitOnce.Do(func() {
		esmiInitOk = (C.esmi_init_wrapper() == 0)
	})
	return esmiInitOk
}
