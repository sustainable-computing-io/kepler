package collector

import (
	"fmt"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"
)

type energyZone struct {
	sync.Mutex

	sysfs sysFS
	desc  *prom.Desc
}

var _ prom.Collector = &energyZone{}

func NewEnergyZoneCollector(sysPath string) (*energyZone, error) {
	sysfs, err := newSysFS(sysPath)
	if err != nil {
		return nil, fmt.Errorf("creating sysfs failed: %w", err)
	}
	return newEnergyCollectorWithFS(sysfs), nil
}

// newEnergyCollectorWithFS injects a sysFS interface
func newEnergyCollectorWithFS(fs sysFS) *energyZone {
	return &energyZone{
		sysfs: fs,
		desc: prom.NewDesc(
			prom.BuildFQName(namespace, "node", "rapl_zone"),
			"Rapl Zones from sysfs",
			[]string{"name", "index", "path"},
			nil,
		),
	}
}

func (e *energyZone) Describe(ch chan<- *prom.Desc) {
	ch <- e.desc
}

func (e *energyZone) Collect(ch chan<- prom.Metric) {
	e.Lock()
	defer e.Unlock()

	zones, err := e.sysfs.Zones()
	if err != nil {
		return
	}
	for _, z := range zones {
		ch <- prom.MustNewConstMetric(
			e.desc,
			prom.GaugeValue,
			1,
			z.Name,
			fmt.Sprintf("%d", z.Index),
			z.Path,
		)
	}
}
