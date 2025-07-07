// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strings"
)

// Level represents the metrics level configuration using bit patterns
type Level uint32

const (
	// Individual metric levels using bit patterns
	MetricsLevelNode      Level = 1 << iota // 1
	MetricsLevelProcess                     // 2
	MetricsLevelContainer                   // 4
	MetricsLevelVM                          // 8
	MetricsLevelPod                         // 16

	// MetricsLevelAll represents all metric levels combined
	MetricsLevelAll = MetricsLevelNode | MetricsLevelProcess | MetricsLevelContainer | MetricsLevelVM | MetricsLevelPod
)

// String returns the string representation of the level
func (l Level) String() string {
	var levels []string
	if l.IsNodeEnabled() {
		levels = append(levels, "node")
	}
	if l.IsProcessEnabled() {
		levels = append(levels, "process")
	}
	if l.IsContainerEnabled() {
		levels = append(levels, "container")
	}
	if l.IsVMEnabled() {
		levels = append(levels, "vm")
	}
	if l.IsPodEnabled() {
		levels = append(levels, "pod")
	}
	return strings.Join(levels, ",")
}

// IsNodeEnabled checks if node metrics are enabled
func (l Level) IsNodeEnabled() bool {
	return l&MetricsLevelNode != 0
}

// IsProcessEnabled checks if process metrics are enabled
func (l Level) IsProcessEnabled() bool {
	return l&MetricsLevelProcess != 0
}

// IsContainerEnabled checks if container metrics are enabled
func (l Level) IsContainerEnabled() bool {
	return l&MetricsLevelContainer != 0
}

// IsVMEnabled checks if VM metrics are enabled
func (l Level) IsVMEnabled() bool {
	return l&MetricsLevelVM != 0
}

// IsPodEnabled checks if pod metrics are enabled
func (l Level) IsPodEnabled() bool {
	return l&MetricsLevelPod != 0
}

// ParseLevel parses a slice of strings into a Level
func ParseLevel(levels []string) (Level, error) {
	if len(levels) == 0 {
		return MetricsLevelAll, nil
	}

	var result Level
	for _, level := range levels {
		switch strings.ToLower(strings.TrimSpace(level)) {
		case "node":
			result |= MetricsLevelNode
		case "process":
			result |= MetricsLevelProcess
		case "container":
			result |= MetricsLevelContainer
		case "vm":
			result |= MetricsLevelVM
		case "pod":
			result |= MetricsLevelPod
		default:
			return 0, fmt.Errorf("unknown metrics level: %s", level)
		}
	}

	return result, nil
}

// ValidLevels returns the list of valid metrics levels
func ValidLevels() []string {
	return []string{"node", "process", "container", "vm", "pod"}
}

// MarshalYAML implements yaml.Marshaler interface
func (l Level) MarshalYAML() (interface{}, error) {
	var levels []string
	if l.IsNodeEnabled() {
		levels = append(levels, "node")
	}
	if l.IsProcessEnabled() {
		levels = append(levels, "process")
	}
	if l.IsContainerEnabled() {
		levels = append(levels, "container")
	}
	if l.IsVMEnabled() {
		levels = append(levels, "vm")
	}
	if l.IsPodEnabled() {
		levels = append(levels, "pod")
	}

	// Return as slice for multiple levels, single string for one level
	if len(levels) == 1 {
		return levels[0], nil
	}
	return levels, nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface
func (l *Level) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a string first
	var single string
	if err := unmarshal(&single); err == nil {
		parsed, parseErr := ParseLevel([]string{single})
		if parseErr != nil {
			return parseErr
		}
		*l = parsed
		return nil
	}

	// Try to unmarshal as a slice of strings
	var multiple []string
	if err := unmarshal(&multiple); err == nil {
		parsed, parseErr := ParseLevel(multiple)
		if parseErr != nil {
			return parseErr
		}
		*l = parsed
		return nil
	}

	return fmt.Errorf("cannot unmarshal metrics level: must be a string or array of strings")
}
