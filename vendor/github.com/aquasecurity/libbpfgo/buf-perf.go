package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"syscall"
)

//
// PerfBuffer
//

type PerfBuffer struct {
	pb         *C.struct_perf_buffer
	bpfMap     *BPFMap
	slot       uint
	eventsChan chan []byte
	lostChan   chan uint64
	stop       chan struct{}
	closed     bool
	wg         sync.WaitGroup
}

// Poll will wait until timeout in milliseconds to gather
// data from the perf buffer.
func (pb *PerfBuffer) Poll(timeout int) {
	pb.stop = make(chan struct{})
	pb.wg.Add(1)
	go pb.poll(timeout)
}

// Deprecated: use PerfBuffer.Poll() instead.
func (pb *PerfBuffer) Start() {
	pb.Poll(300)
}

func (pb *PerfBuffer) Stop() {
	if pb.stop == nil {
		return
	}

	// Signal the poll goroutine to exit
	close(pb.stop)

	// The event and lost channels should be drained here since the consumer
	// may have stopped at this point. Failure to drain it will
	// result in a deadlock: the channel will fill up and the poll
	// goroutine will block in the callback.
	go func() {
		// revive:disable:empty-block
		for range pb.eventsChan {
		}

		if pb.lostChan != nil {
			for range pb.lostChan {
			}
		}
		// revive:enable:empty-block
	}()

	// Wait for the poll goroutine to exit
	pb.wg.Wait()

	// Close the channel -- this is useful for the consumer but
	// also to terminate the drain goroutine above.
	close(pb.eventsChan)
	if pb.lostChan != nil {
		close(pb.lostChan)
	}

	// Reset pb.stop to allow multiple safe calls to Stop()
	pb.stop = nil
}

func (pb *PerfBuffer) Close() {
	if pb.closed {
		return
	}

	pb.Stop()
	C.perf_buffer__free(pb.pb)
	eventChannels.remove(pb.slot)
	pb.closed = true
}

// todo: consider writing the perf polling in go as c to go calls (callback) are expensive
func (pb *PerfBuffer) poll(timeout int) error {
	defer pb.wg.Done()

	for {
		select {
		case <-pb.stop:
			return nil
		default:
			retC := C.perf_buffer__poll(pb.pb, C.int(timeout))
			if retC < 0 {
				errno := syscall.Errno(-retC)
				if errno == syscall.EINTR {
					continue
				}

				return fmt.Errorf("error polling perf buffer: %w", errno)
			}
		}
	}
}
