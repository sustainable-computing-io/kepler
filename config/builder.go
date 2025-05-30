// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"reflect"

	"dario.cat/mergo"
	"gopkg.in/yaml.v3"
)

// Builder is a struct for building a config
type Builder struct {
	yamls  []string
	Config *Config
}

// Use sets the default configuration
func (b *Builder) Use(c *Config) *Builder {
	b.Config = c
	return b
}

// Merge adds a YAML string to be merged into the configuration
func (b *Builder) Merge(yamls ...string) *Builder {
	b.yamls = append(b.yamls, yamls...)
	return b
}

// Build constructs the final configuration by merging all additional YAMLS into the default configuration
func (b *Builder) Build() (*Config, error) {
	if b.Config == nil {
		b.Config = DefaultConfig()
	}

	var errs error
	for _, y := range b.yamls {
		additional := &Config{}
		if err := yaml.Unmarshal([]byte(y), additional); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to parse YAML: %w, yaml: %s", err, y))
			continue
		}

		if err := mergo.Merge(b.Config, additional, mergo.WithOverride, mergo.WithTransformers(boolPtrTransformer{})); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to merge config: %w, yaml: %s", err, y))
			continue
		}
	}

	if errs != nil {
		return nil, errs
	}
	return b.Config, nil
}

// boolPtrTransformer is a custom transformer for merging boolean types.
type boolPtrTransformer struct{}

func (t boolPtrTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ != reflect.TypeOf((*bool)(nil)) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		if src.IsNil() {
			return nil
		}
		if dst.CanSet() {
			dst.Set(src)
		}
		return nil
	}
}
