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
	"fmt"
	"os"
	"os/signal"

	"github.com/oklog/run"
)

func main() {

	var g run.Group

	fmt.Println("Starting Kepler...")

	ctx, cancel := context.WithCancel(context.Background())
	{
		g.Add(waitForInterrupt(ctx, os.Interrupt))
	}

	{
		// TODO: replace with monitor.Start()
		g.Add(
			func() error {
				fmt.Println("Monitor is running. Press Ctrl+C to stop.")
				<-ctx.Done()
				fmt.Println("Monitor is done running.")
				return nil
			},
			func(err error) {
				fmt.Println("Shutting down...:", err)
				cancel()
			},
		)
	}

	{

		// TODO: replace with server.Start()
		g.Add(
			func() error {
				fmt.Println("HTTP server is running. Press Ctrl+C to stop.")
				<-ctx.Done()
				return nil
			},
			func(err error) {
				fmt.Println("HTTP Server: Shutting down...:", err)
				cancel()
			},
		)
	}

	// run all groups
	if err := g.Run(); err != nil {
		fmt.Printf("Kepler terminated with error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Graceful shutdown completed")
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
