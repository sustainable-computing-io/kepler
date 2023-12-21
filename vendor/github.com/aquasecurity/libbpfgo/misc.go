package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

//
// Misc generic helpers
//

// roundUp rounds x up to the nearest multiple of y.
func roundUp(x, y uint64) uint64 {
	return ((x + (y - 1)) / y) * y
}
