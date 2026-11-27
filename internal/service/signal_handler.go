// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
)

type SignalHandler struct {
	logger  *slog.Logger
	signals []os.Signal
}

func NewSignalHandler(logger *slog.Logger, signals ...os.Signal) *SignalHandler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return &SignalHandler{
		logger:  logger,
		signals: signals,
	}
}

func (sh *SignalHandler) Name() string {
	return "signal-handler"
}

func (sh *SignalHandler) Run(ctx context.Context) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sh.signals...)
	sh.logger.Info("Press Ctrl+C to shutdown")

	select {
	case <-c:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}
