package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/bdtfs/gnat/internal/di"
)

const (
	successExitCode = 0
	failExitCode    = 1
)

func main() {
	os.Exit(run())
}

func run() int {
	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(os.Stderr, "panic: %v\n", r)
			debug.PrintStack()
			os.Exit(failExitCode)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c := di.New(ctx)

	printWelcome(":8080")

	errChan := make(chan error, 1)
	go func() {
		errChan <- c.GetServer().Start(ctx)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			return failExitCode
		}
	case <-ctx.Done():
		fmt.Println("\nShutdown signal received, stopping server...")
	}

	return successExitCode
}
