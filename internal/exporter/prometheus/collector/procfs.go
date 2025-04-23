package collector

import (
	"github.com/prometheus/procfs"
)

// procFS is an interface to prometheus/procfs
type procFS interface {
	CPUInfo() ([]procfs.CPUInfo, error)
}

type realProcFS struct {
	fs procfs.FS
}

func (r *realProcFS) CPUInfo() ([]procfs.CPUInfo, error) {
	return r.fs.CPUInfo()
}

func newProcFS(mountPoint string) (procFS, error) {
	fs, err := procfs.NewFS(mountPoint)
	if err != nil {
		return nil, err
	}
	return &realProcFS{fs: fs}, nil
}
