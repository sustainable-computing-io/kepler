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

	interval     time.Duration
	clock        clock.WithTicker
	maxStaleness time.Duration

	// signals when a snapshot has been updated
	dataCh chan struct{}

	computeGroup singleflight.Group
	snapshot     atomic.Pointer[Snapshot]

	zonesNames []string // cache of all zones

	// For managing the collection loop
	collectionCtx    context.Context
	collectionCancel context.CancelFunc
}

var _ Service = (*PowerMonitor)(nil)

// NewPowerMonitor creates a new PowerMonitor instance
func NewPowerMonitor(meter device.CPUPowerMeter, applyOpts ...OptionFn) *PowerMonitor {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &PowerMonitor{
		logger:           opts.logger.With("service", "monitor"),
		cpu:              meter,
		clock:            opts.clock,
		interval:         opts.interval,
		dataCh:           make(chan struct{}, 1),
		maxStaleness:     opts.maxStaleness,
		collectionCtx:    ctx,
		collectionCancel: cancel,
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
	case pm.dataCh <- struct{}{}: // send signal to any waiting goroutine
		pm.logger.Debug("Data channel updated")
	default:
		pm.logger.Debug("Data channel is full")
	}
}

func (pm *PowerMonitor) Run(ctx context.Context) error {
	pm.logger.Info("Monitor is running...")
	pm.collectionLoop()
	<-ctx.Done()
	pm.collectionCancel()
	pm.logger.Info("Monitor has terminated.")
	return nil
}

func (pm *PowerMonitor) Shutdown() error {
	pm.logger.Info("shutting down monitor")
	pm.collectionCancel()
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

// collectionLoop handles periodic data collection
func (pm *PowerMonitor) collectionLoop() {
	if err := pm.synchronizedPowerRefresh(); err != nil {
		pm.logger.Error("Failed to collect initial power data", "error", err)
	}

	if pm.interval > 0 {
		pm.scheduleNextCollection()
	}
}

// scheduleNextCollection schedules the next data collection
func (pm *PowerMonitor) scheduleNextCollection() {
	timer := pm.clock.After(pm.interval)
	go func() {
		select {
		case <-timer:
			if err := pm.synchronizedPowerRefresh(); err != nil {
				pm.logger.Error("Failed to collect power data", "error", err)
			}
			pm.scheduleNextCollection()

		case <-pm.collectionCtx.Done():
			pm.logger.Info("Collection loop terminated")
			return
		}
	}()
}

// ensureFreshData ensures that the data returned is recent enough (< maxStaleness)
func (pm *PowerMonitor) ensureFreshData() error {
	if pm.isFresh() {
		return nil // Data is fresh, nothing more to do
	}

	return pm.synchronizedPowerRefresh()
}

// synchronizedPowerRefresh creates a new snapshot of power consumption, while
// ensuring that only one go routine does computation at a time.
// This is called by the scheduleNextCollection and by ensureFreshData
func (pm *PowerMonitor) synchronizedPowerRefresh() error {
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

		return nil, pm.refreshSnapshot()
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

// refreshSnapshot create a new snapshot of the power consumption of various levels( currently only node)
func (pm *PowerMonitor) refreshSnapshot() error {
	started := pm.clock.Now()
	defer func() { pm.logger.Info("Computed power", "duration", pm.clock.Since(started)) }()

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
	newSnapshot.Timestamp = pm.clock.Now()
	pm.snapshot.Store(newSnapshot)

	pm.signalNewData()

	return nil
}
