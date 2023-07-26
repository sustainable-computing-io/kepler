package libbpfgo

/*
#include <bpf/libbpf.h>
*/
import "C"

import (
	"fmt"
	"os"
)

// This callback definition needs to be in a different file from where it is
// declared in C Otherwise, multiple definition compilation error will occur

// loggerCallback is called by libbpf_print_fn() which in turn is called by
// libbpf
//
// revive:disable
//
//export loggerCallback
func loggerCallback(libbpfPrintLevel int, libbpfOutput *C.char) {
	goOutput := C.GoString(libbpfOutput)

	for _, fnFilterOut := range callbacks.LogFilters {
		if fnFilterOut != nil {
			if fnFilterOut(libbpfPrintLevel, goOutput) {
				return
			}
		}
	}

	// pass received output to callback, leaving formatting to consumer
	callbacks.Log(libbpfPrintLevel, goOutput)
}

const (
	// libbpf print levels
	LibbpfWarnLevel  = int(C.LIBBPF_WARN)
	LibbpfInfoLevel  = int(C.LIBBPF_INFO)
	LibbpfDebugLevel = int(C.LIBBPF_DEBUG)
)

// Callbacks stores the callbacks to be used by libbpfgo
type Callbacks struct {
	Log        func(level int, msg string)
	LogFilters []func(libLevel int, msg string) bool
}

// callbacks is initialized with default callbacks, but can be changed by SetLoggerCbs
var callbacks = Callbacks{
	Log:        logFallback,
	LogFilters: []func(libLevel int, msg string) bool{},
}

// SetLoggerCbs receives Callbacks type to be used to log libbpf outputs and to filter out those outputs
func SetLoggerCbs(cbs Callbacks) {
	if cbs.Log == nil { // guarantee that there is always an outputter
		cbs.Log = logFallback
	}

	callbacks = cbs
}

// logFallback is the default logger callback
// - level is ignored
func logFallback(level int, msg string) {
	fmt.Fprint(os.Stderr, msg)
}

// revive:enable
