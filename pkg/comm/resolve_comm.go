package comm

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const unknownComm = "unknown"

type CommResolver struct {
	cacheExist     map[int]string
	cacheNotExist  map[int]struct{}
	procFsResolver func(pid int) (string, error)
}

func NewCommResolver() *CommResolver {
	return &CommResolver{
		cacheExist:     map[int]string{},
		cacheNotExist:  map[int]struct{}{},
		procFsResolver: readCommandFromProcFs,
	}
}

func NewTestCommResolver(procFsResolver func(pid int) (string, error)) *CommResolver {
	return &CommResolver{
		cacheExist:     map[int]string{},
		cacheNotExist:  map[int]struct{}{},
		procFsResolver: procFsResolver,
	}
}

func (r *CommResolver) ResolveComm(pid int) (string, error) {
	if comm, ok := r.cacheExist[pid]; ok {
		return comm, nil
	}
	if _, ok := r.cacheNotExist[pid]; ok {
		return unknownComm, fmt.Errorf("process not running")
	}

	comm, err := r.procFsResolver(pid)
	if err != nil && os.IsNotExist(err) {
		// skip process that is not running
		r.cacheNotExist[pid] = struct{}{}
		return unknownComm, fmt.Errorf("process not running: %w", err)
	}

	r.cacheExist[pid] = comm
	return comm, nil
}

func (r *CommResolver) Clear(freed []int) {
	for _, pid := range freed {
		delete(r.cacheExist, pid)
		delete(r.cacheNotExist, pid)
	}
}

func readCommandFromProcFs(pid int) (string, error) {
	if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); os.IsNotExist(err) {
		return "", err
	}
	var comm string
	if cmdLineBytes, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline"); err == nil {
		comm = readCommandFromProcFsCmdline(cmdLineBytes)
	}
	if comm != "" {
		return comm, nil
	}
	if commBytes, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm"); err == nil {
		comm = readCommandFromProcFsComm(commBytes)
	}
	if comm != "" {
		return comm, nil
	}
	return unknownComm, nil
}

// This gives the same output as `ps -o comm` command
func readCommandFromProcFsCmdline(b []byte) string {
	// replace null bytes with new line
	buf := bytes.ReplaceAll(b, []byte{0x0}, []byte{0x0a})
	// Using all the parts would be nice, but as these become prometheus labels
	// we need to be careful about the cardinality. Just use the first part.
	parts := strings.Split(strings.TrimSpace(unix.ByteSliceToString(buf)), "\n")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

// This is a fallback method when we can't read the executable name from
// the cmdline. i.e for kernel threads
func readCommandFromProcFsComm(b []byte) string {
	comm := strings.TrimSpace(unix.ByteSliceToString(b))
	if comm != "" {
		// return the command in square brackets, like ps does
		return "[" + comm + "]"
	}
	return ""
}
