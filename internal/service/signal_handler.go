// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

type SignalHandler struct {
	signals []os.Signal
}

func NewSignalHandler(signals ...os.Signal) *SignalHandler {
	return &SignalHandler{
		signals: signals,
	}
}

func (sh *SignalHandler) Name() string {
	return "signal-handler"
}

func (sh *SignalHandler) Run(ctx context.Context) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sh.signals...)
	fmt.Println("Press Ctrl+C to shutdown")

	select {
	case <-c:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}
