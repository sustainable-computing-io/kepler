// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import "context"

// mockService implements Service interface
type mockService struct {
	name string
}

func (m *mockService) Name() string {
	return m.name
}

// mockInitializer implements Initializer interface
type mockInitializer struct {
	mockService
	initFn    func() error
	initCount int
}

func (m *mockInitializer) Init() error {
	m.initCount++
	if m.initFn != nil {
		return m.initFn()
	}
	return nil
}

// mockInitShutdownService implements both Initializer and Shutdowner
type mockInitShutdownService struct {
	mockService
	initFn        func() error
	shutdownFn    func() error
	initCount     int
	shutdownCount int
}

func (m *mockInitShutdownService) Init() error {
	m.initCount++
	if m.initFn != nil {
		return m.initFn()
	}
	return nil
}

func (m *mockInitShutdownService) Shutdown() error {
	m.shutdownCount++
	if m.shutdownFn != nil {
		return m.shutdownFn()
	}
	return nil
}

// mockRunner implements Runner interface
type mockRunner struct {
	mockService
	runFn    func(ctx context.Context) error
	runCount int
}

func (m *mockRunner) Run(ctx context.Context) error {
	m.runCount++
	if m.runFn != nil {
		return m.runFn(ctx)
	}
	return nil
}

// mockRunShutdownService implements both Runner and Shutdowner
type mockRunShutdownService struct {
	mockService
	runFn         func(ctx context.Context) error
	shutdownFn    func() error
	runCount      int
	shutdownCount int
}

func (m *mockRunShutdownService) Run(ctx context.Context) error {
	m.runCount++
	if m.runFn != nil {
		return m.runFn(ctx)
	}
	return nil
}

func (m *mockRunShutdownService) Shutdown() error {
	m.shutdownCount++
	if m.shutdownFn != nil {
		return m.shutdownFn()
	}
	return nil
}
