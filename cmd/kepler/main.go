/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/oklog/run"
	"github.com/sustainable-computing-io/kepler/internal/version"
)

func main() {
	logger := setupLogger("info", "text")
	logVersionInfo(logger)

	var g run.Group

	logger.Info("Starting Kepler...")
	ctx, cancel := context.WithCancel(context.Background())
	{
		g.Add(waitForInterrupt(ctx, os.Interrupt))
	}

	{
		// TODO: replace with monitor.Start()
		g.Add(
			func() error {
				logger.Info("Monitor is running. Press Ctrl+C to stop.")
				<-ctx.Done()
				logger.Info("Monitor is done running.")
				return nil
			},
			func(err error) {
				logger.Warn("Shutting down...:", "error", err)
				cancel()
			},
		)
	}

	{
		// TODO: replace with server.Start()
		g.Add(
			func() error {
				logger.Info("HTTP server is running. Press Ctrl+C to stop.")
				<-ctx.Done()
				return nil
			},
			func(err error) {
				logger.Info("HTTP Server: Shutting down...:", "error", err)
				cancel()
			},
		)
	}

	// run all groups
	if err := g.Run(); err != nil {
		logger.Warn("Kepler terminated with error: %v\n", "error", err)
		os.Exit(1)
	}
	logger.Info("Graceful shutdown completed")
}

func logVersionInfo(logger *slog.Logger) {
	v := version.Info()
	logger.Info("Kepler version information",
		"version", v.Version,
		"buildTime", v.BuildTime,
		"gitBranch", v.GitBranch,
		"gitCommit", v.GitCommit,
		"goVersion", v.GoVersion,
		"goOS", v.GoOS,
		"goArch", v.GoArch,
	)
}

func waitForInterrupt(ctx context.Context, signals ...os.Signal) (func() error, func(error)) {
	ctx, cancel := context.WithCancel(ctx)
	return func() error {
			c := make(chan os.Signal, 1)
			signal.Notify(c, signals...)
			select {
			case <-c:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}, func(error) {
			cancel()
		}
}
