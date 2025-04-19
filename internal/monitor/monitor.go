// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/service"
	"golang.org/x/sync/singleflight"
	"k8s.io/utils/clock"
)

type PowerDataProvider interface {
	// Snapshot returns the current power data
	Snapshot() (*Snapshot, error)

	// DataChannel returns a channel that signals when new data is available
	DataChannel() <-chan struct{}

	// ZoneNames returns the names of the available RAPL zones
	ZoneNames() []string
}

// Service defines the interface for the power monitoring service
type Service interface {
	service.Service
	PowerDataProvider
}

// PowerMonitor is the default implementation of the monitoring service
type PowerMonitor struct {
	// passed externally
	logger *slog.Logger
	cpu    device.CPUPowerMeter

	clock clock.WithTicker

	// signals when a snapshot has been updated
	dataCh chan struct{}

	computeGroup singleflight.Group
	maxStaleness time.Duration

	zones []string // cache of all zones read

	snapshotMu sync.RWMutex
	snapshot   *Snapshot
}

var _ Service = (*PowerMonitor)(nil)

// NewPowerMonitor creates a new PowerMonitor instance
func NewPowerMonitor(meter device.CPUPowerMeter, applyOpts ...OptionFn) *PowerMonitor {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	logger := opts.logger.With("service", "monitor")

	monitor := &PowerMonitor{
		logger:       logger,
		cpu:          meter,
		clock:        opts.clock,
		dataCh:       make(chan struct{}, 1),
		snapshot:     NewSnapshot(),
		maxStaleness: opts.maxStaleness,
	}

	return monitor
}

func (pm *PowerMonitor) Name() string {
	return "monitor"
}

func (pm *PowerMonitor) Init(ctx context.Context) error {
	if err := pm.cpu.Init(ctx); err != nil {
		return fmt.Errorf("failed to start cpu power meter: %w", err)
	}

	// zone names need to be collected once and can be cached
	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	pm.zones = make([]string, len(zones))
	for i, zone := range zones {
		pm.zones[i] = zone.Name()
	}
	pm.signalNewData()

	return nil
}

func (pm *PowerMonitor) signalNewData() {
	select {
	case pm.dataCh <- struct{}{}: // send signal to any waiting goroutinel
	default:
	}
}

func (pm *PowerMonitor) Run(ctx context.Context) error {
	pm.logger.Info("Monitor is running...")
	<-ctx.Done()
	pm.logger.Info("Monitor has terminated.")

	return nil
}

func (pm *PowerMonitor) Shutdown() error {
	return pm.cpu.Stop()
}

func (pm *PowerMonitor) DataChannel() <-chan struct{} {
	return pm.dataCh
}

func (pm *PowerMonitor) ZoneNames() []string {
	return pm.zones
}

func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
	if err := pm.ensureFreshData(); err != nil {
		return nil, err
	}

	pm.snapshotMu.RLock()
	defer pm.snapshotMu.RUnlock()
	return pm.snapshot.Clone(), nil
}

func (pm *PowerMonitor) isFresh() bool {
	pm.snapshotMu.RLock()
	defer pm.snapshotMu.RUnlock()

	if pm.snapshot == nil || pm.snapshot.Timestamp.IsZero() {
		return false
	}

	age := pm.clock.Now().Sub(pm.snapshot.Timestamp)
	return age <= pm.maxStaleness
}

// ensureFreshData ensures that the data returned is recent enough (< maxStaleness)
func (pm *PowerMonitor) ensureFreshData() error {
	if pm.isFresh() {
		return nil // Data is fresh, nothing more to do
	}
	// NOTE: ensure that only one goroutine is computing power even if multiple calls are made
	_, err, _ := pm.computeGroup.Do("compute", func() (any, error) {
		// NOTE: Double-check freshness after acquiring singleflight lock
		//
		//  The reason this double checking pattern is required is to mitigate the following scenario
		//
		//  *** Without double-checking ***
		//      Go Routine 1          |   Go Routine 2
		//    ------------------------------------------------------
		//    isFresh? -> false       |  isFresh? -> false
		//    acquires the lock 🔐    |  waits for the lock
		//    updates the data,       |
		//    releases the lock       |  ...
		//                            |  acquires the lock 🔐
		//                            |  updates the data,
		//                            |  releases the lock
		//
		// With double-checking:
		//      Go Routine 1          |   Go Routine 2
		//    ------------------------------------------------------
		//    isFresh? -> false       |  isFresh? -> false
		//    acquires the lock 🔐    |  waits for the lock
		//    updates the data,       |
		//    releases the lock       | ...
		//                            |  acquires the lock 🔐
		//                            |  isFresh? -> true ✅
		//                            |  releases the lock
		if pm.isFresh() {
			return nil, nil
		}

		return nil, pm.computePower()
	})

	return err
}

// computePower create a new snapshot of the power consumption of various levels( currently only node)
func (pm *PowerMonitor) computePower() error {
	started := time.Now()
	defer func() { pm.logger.Info("Computed power", "duration", time.Since(started)) }()

	newSnapshot := NewSnapshot()
	if err := pm.calculateNodePower(newSnapshot); err != nil {
		return fmt.Errorf("failed to calculate node power %w", err)
	}

	// update snapshot
	newSnapshot.Timestamp = time.Now()
	pm.snapshotMu.Lock()
	pm.snapshot = newSnapshot
	pm.snapshotMu.Unlock()

	pm.signalNewData()

	return nil
}
