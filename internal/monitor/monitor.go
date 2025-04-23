// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
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

	clock        clock.WithTicker
	maxStaleness time.Duration

	// signals when a snapshot has been updated
	dataCh chan struct{}

	computeGroup singleflight.Group
	snapshot     atomic.Pointer[Snapshot]

	zonesNames []string // cache of all zones
}

var _ Service = (*PowerMonitor)(nil)

// NewPowerMonitor creates a new PowerMonitor instance
func NewPowerMonitor(meter device.CPUPowerMeter, applyOpts ...OptionFn) *PowerMonitor {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	monitor := &PowerMonitor{
		logger:       opts.logger.With("service", "monitor"),
		cpu:          meter,
		clock:        opts.clock,
		dataCh:       make(chan struct{}, 1),
		maxStaleness: opts.maxStaleness,
	}

	return monitor
}

func (pm *PowerMonitor) Name() string {
	return "monitor"
}

func (pm *PowerMonitor) Init() error {
	if err := pm.initZones(); err != nil {
		return fmt.Errorf("zone initialization failed: %w", err)
	}
	// signal now so that exporters can construct descriptors
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

func (pm *PowerMonitor) DataChannel() <-chan struct{} {
	return pm.dataCh
}

func (pm *PowerMonitor) ZoneNames() []string {
	// need not lock since it is read-only
	return pm.zonesNames
}

func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
	if err := pm.ensureFreshData(); err != nil {
		return nil, err
	}

	snapshot := pm.snapshot.Load()
	if snapshot == nil {
		return nil, fmt.Errorf("failed to get snapshot")
	}
	return snapshot.Clone(), nil
}

func (pm *PowerMonitor) initZones() error {
	// zone names need to be collected only once and can be cached
	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	pm.zonesNames = make([]string, len(zones))
	for i, zone := range zones {
		pm.zonesNames[i] = zone.Name()
	}

	return nil
}

// ensureFreshData ensures that the data returned is recent enough (< maxStaleness)
func (pm *PowerMonitor) ensureFreshData() error {
	if pm.isFresh() {
		return nil // Data is fresh, nothing more to do
	}

	return pm.collectData()
}

// collectData creates a new snapshot of power consumption
// This is called by ensureFresh when the snapshot is stale
func (pm *PowerMonitor) collectData() error {
	// Use singleflight to ensure only one go routine does computation at a time

	_, err, _ := pm.computeGroup.Do("compute", func() (any, error) {
		// NOTE: (Double) check freshness after acquiring singleflight lock
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

func (pm *PowerMonitor) isFresh() bool {
	snapshot := pm.snapshot.Load()
	if snapshot == nil || snapshot.Timestamp.IsZero() {
		return false
	}

	age := pm.clock.Now().Sub(snapshot.Timestamp)
	return age <= pm.maxStaleness
}

// computePower create a new snapshot of the power consumption of various levels( currently only node)
func (pm *PowerMonitor) computePower() error {
	started := time.Now()
	defer func() { pm.logger.Info("Computed power", "duration", time.Since(started)) }()

	prevSnapshot := pm.snapshot.Load()
	// ensure snapshot is not nil from here on
	if prevSnapshot == nil {
		prevSnapshot = NewSnapshot()
	}

	newSnapshot := NewSnapshot()

	if err := pm.calculateNodePower(newSnapshot.Node, prevSnapshot.Node); err != nil {
		return fmt.Errorf("failed to calculate node power %w", err)
	}

	// update snapshot
	newSnapshot.Timestamp = time.Now()
	pm.snapshot.Store(newSnapshot)

	pm.signalNewData()

	return nil
}
