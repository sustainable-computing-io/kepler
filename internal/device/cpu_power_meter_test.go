// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCPUPowerMeterInterface ensures that cpuPowerMeter properly implements the CPUPowerMeter interface
func TestCPUPowerMeterInterface(t *testing.T) {
	var _ CPUPowerMeter = (*cpuPowerMeter)(nil)
}

func TestNewCPUPowerMeter(t *testing.T) {
	meter := NewCPUPowerMeter()
	assert.NotNil(t, meter, "NewCPUPowerMeter should not return nil")
	assert.IsType(t, &cpuPowerMeter{}, meter, "NewCPUPowerMeter should return a *cpuPowerMeter")
}

func TestCPUPowerMeter_Name(t *testing.T) {
	meter := &cpuPowerMeter{}
	name := meter.Name()
	assert.Equal(t, "cpu", name, "Name() should return 'cpu'")
}

func TestCPUPowerMeter_Start(t *testing.T) {
	meter := &cpuPowerMeter{}
	ctx := context.Background()
	err := meter.Start(ctx)
	assert.NoError(t, err, "Start() should not return an error")
}

func TestCPUPowerMeter_Stop(t *testing.T) {
	meter := &cpuPowerMeter{}
	err := meter.Stop()
	assert.NoError(t, err, "Stop() should not return an error")
}

func TestCPUPowerMeter_Zones(t *testing.T) {
	meter := &cpuPowerMeter{}
	zones, err := meter.Zones()
	assert.NoError(t, err, "Zones() should not return an error")
	assert.Nil(t, zones, "Zones() should return nil for the current implementation")
}
