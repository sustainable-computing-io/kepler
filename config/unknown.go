// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"log/slog"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// notFoundRe matches the yaml.v3 strict-decoding message for a key that has no
// matching struct field, e.g.
// "line 12: field dcgmEndpoint not found in type config.GPU".
var notFoundRe = regexp.MustCompile(`field (\S+) not found in type`)

// unknownFields returns the config keys that the decoder ignored.
//
// Load uses a permissive decode so that an unknown key never prevents Kepler
// from starting. That also means a typo, or an option belonging to a different
// release, disappears without a trace. A second strict pass collects those keys
// so they can be reported.
func unknownFields(data []byte) []string {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	err := dec.Decode(&Config{})
	if err == nil {
		return nil
	}

	typeErr, ok := err.(*yaml.TypeError)
	if !ok {
		return nil
	}

	seen := map[string]struct{}{}
	fields := []string{}
	for _, msg := range typeErr.Errors {
		m := notFoundRe.FindStringSubmatch(msg)
		if m == nil {
			continue
		}
		if _, dup := seen[m[1]]; dup {
			continue
		}
		seen[m[1]] = struct{}{}
		fields = append(fields, m[1])
	}
	sort.Strings(fields)

	return fields
}

// LogUnknownFields warns about config keys that were ignored while loading.
// Unknown keys are not fatal, so an existing config that used to work keeps
// working.
func (c *Config) LogUnknownFields(logger *slog.Logger) {
	if len(c.unknownFields) == 0 {
		return
	}
	logger.Warn("ignoring unknown config fields; check for typos or options from another release",
		"fields", c.unknownFields)
}
