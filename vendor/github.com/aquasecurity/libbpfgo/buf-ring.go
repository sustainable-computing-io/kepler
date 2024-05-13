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
// RingBuffer
//

type RingBuffer struct {
	rb     *C.struct_ring_buffer
	bpfMap *BPFMap
	slot   uint
	stop   chan struct{}
	closed bool
	wg     sync.WaitGroup
}

// Poll will wait until timeout in milliseconds to gather
// data from the ring buffer.
func (rb *RingBuffer) Poll(timeout int) {
	rb.stop = make(chan struct{})
	rb.wg.Add(1)
	go rb.poll(timeout)
}

// Deprecated: use RingBuffer.Poll() instead.
func (rb *RingBuffer) Start() {
	rb.Poll(300)
}

func (rb *RingBuffer) Stop() {
	if rb.stop == nil {
		return
	}

	// Signal the poll goroutine to exit
	close(rb.stop)

	// The event channel should be drained here since the consumer
	// may have stopped at this point. Failure to drain it will
	// result in a deadlock: the channel will fill up and the poll
	// goroutine will block in the callback.
	eventChan := eventChannels.get(rb.slot).(chan []byte)
	go func() {
		// revive:disable:empty-block
		for range eventChan {
		}
		// revive:enable:empty-block
	}()

	// Wait for the poll goroutine to exit
	rb.wg.Wait()

	// Close the channel -- this is useful for the consumer but
	// also to terminate the drain goroutine above.
	close(eventChan)

	// Reset pb.stop to allow multiple safe calls to Stop()
	rb.stop = nil
}

func (rb *RingBuffer) Close() {
	if rb.closed {
		return
	}

	rb.Stop()
	C.ring_buffer__free(rb.rb)
	eventChannels.remove(rb.slot)
	rb.closed = true
}

func (rb *RingBuffer) isStopped() bool {
	select {
	case <-rb.stop:
		return true
	default:
		return false
	}
}

func (rb *RingBuffer) poll(timeout int) error {
	defer rb.wg.Done()

	for {
		retC := C.ring_buffer__poll(rb.rb, C.int(timeout))
		if rb.isStopped() {
			break
		}

		if retC < 0 {
			errno := syscall.Errno(-retC)
			if errno == syscall.EINTR {
				continue
			}

			return fmt.Errorf("error polling ring buffer: %w", errno)
		}
	}

	return nil
}
