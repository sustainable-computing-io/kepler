package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Println("Starting Kepler...")

	// Register signal handler for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signalCh
		fmt.Printf("Received termination signal: %s, shutting down...\n", sig.String())
		cancel()
	}()

	// Add main logic
	// ...

	fmt.Println("Kepler is running. Press Ctrl+C to stop.")

	// Wait for termination signal
	<-ctx.Done()

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Graceful shutdown logic
	// ...

	// Wait for graceful shutdown period
	select {
	case <-shutdownCtx.Done():
		if shutdownCtx.Err() == context.DeadlineExceeded {
			fmt.Println("Graceful shutdown timed out")
		}
	case <-time.After(100 * time.Millisecond): // quick shutdown
		fmt.Println("Kepler stopped successfully")
	}

	fmt.Println("Graceful shutdown completed")
}
