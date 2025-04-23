package collector

import "github.com/prometheus/procfs/sysfs"

// sysFS is an interface to prometheus/procfs/sysfs
type sysFS interface {
	Zones() ([]sysfs.RaplZone, error)
}

type realSysFS struct {
	sysfs sysfs.FS
}

func (s *realSysFS) Zones() ([]sysfs.RaplZone, error) {
	return sysfs.GetRaplZones(s.sysfs)
}

func newSysFS(mountPoint string) (sysFS, error) {
	sysfs, err := sysfs.NewFS(mountPoint)
	if err != nil {
		return nil, err
	}
	return &realSysFS{sysfs: sysfs}, nil
}
