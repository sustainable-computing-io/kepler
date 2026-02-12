// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestApplyGPUConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *Config
		flagsSet   map[string]bool
		enabled    *bool
		idlePower  *float64
		wantExpNil bool
		wantGPU    *ExperimentalGPU // nil means don't check GPU fields
	}{{
		name:       "no flags and no experimental config",
		cfg:        &Config{},
		flagsSet:   map[string]bool{},
		enabled:    ptr.To(false),
		idlePower:  ptr.To(0.0),
		wantExpNil: true,
	}, {
		name:      "gpu enabled flag only",
		cfg:       &Config{},
		flagsSet:  map[string]bool{ExperimentalGPUEnabledFlag: true},
		enabled:   ptr.To(true),
		idlePower: ptr.To(0.0),
		wantGPU: &ExperimentalGPU{
			Enabled:   ptr.To(true),
			IdlePower: 0,
		},
	}, {
		name: "gpu enabled and idle power flags",
		cfg:  &Config{},
		flagsSet: map[string]bool{
			ExperimentalGPUEnabledFlag:   true,
			ExperimentalGPUIdlePowerFlag: true,
		},
		enabled:   ptr.To(true),
		idlePower: ptr.To(50.0),
		wantGPU: &ExperimentalGPU{
			Enabled:   ptr.To(true),
			IdlePower: 50.0,
		},
	}, {
		name: "gpu disabled with idle power flag",
		cfg:  &Config{},
		flagsSet: map[string]bool{
			ExperimentalGPUEnabledFlag:   true,
			ExperimentalGPUIdlePowerFlag: true,
		},
		enabled:   ptr.To(false),
		idlePower: ptr.To(50.0),
		wantGPU: &ExperimentalGPU{
			Enabled:   ptr.To(false),
			IdlePower: 0, // idle power not applied when GPU is disabled
		},
	}, {
		name:       "only idle power flag without enabled flag",
		cfg:        &Config{},
		flagsSet:   map[string]bool{ExperimentalGPUIdlePowerFlag: true},
		enabled:    ptr.To(false),
		idlePower:  ptr.To(50.0),
		wantExpNil: true, // early exit — enabled flag not in flagsSet, Experimental is nil
	}, {
		name: "yaml gpu enabled with idle power flag override",
		cfg: &Config{
			Experimental: &Experimental{
				GPU: ExperimentalGPU{
					Enabled: ptr.To(true),
				},
			},
		},
		flagsSet:  map[string]bool{ExperimentalGPUIdlePowerFlag: true},
		enabled:   ptr.To(false),
		idlePower: ptr.To(25.0),
		wantGPU: &ExperimentalGPU{
			Enabled:   ptr.To(true), // preserved from YAML
			IdlePower: 25.0,
		},
	}, {
		name: "enabled flag overrides yaml disabled",
		cfg: &Config{
			Experimental: &Experimental{
				GPU: ExperimentalGPU{
					Enabled: ptr.To(false),
				},
			},
		},
		flagsSet:  map[string]bool{ExperimentalGPUEnabledFlag: true},
		enabled:   ptr.To(true),
		idlePower: ptr.To(0.0),
		wantGPU: &ExperimentalGPU{
			Enabled:   ptr.To(true),
			IdlePower: 0,
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applyGPUConfig(tc.cfg, tc.flagsSet, tc.enabled, tc.idlePower)

			if tc.wantExpNil {
				assert.Nil(t, tc.cfg.Experimental)
				return
			}

			assert.NotNil(t, tc.cfg.Experimental)
			assert.Equal(t, tc.wantGPU.Enabled, tc.cfg.Experimental.GPU.Enabled)
			assert.Equal(t, tc.wantGPU.IdlePower, tc.cfg.Experimental.GPU.IdlePower)
		})
	}
}
