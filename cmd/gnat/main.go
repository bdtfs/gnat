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

	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	switch cmd {
	case "serve":
		return runServe()
	case "attack":
		return runAttack(args)
	case "run":
		return runRun(args)
	case "version", "--version", "-v":
		fmt.Printf("gnat %s\n", version)
		return successExitCode
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return successExitCode
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		printUsage(os.Stderr)
		return failExitCode
	}
}

func printUsage(w *os.File) {
	_, _ = fmt.Fprint(w, `gnat - load testing tool

Usage:
  gnat <command> [flags]

Commands:
  serve              Run the HTTP API server (default). Port via APPLICATION_PORT (8778).
  attack             Run a config-driven constant-RPS load test and exit.
  run                Run a stateful multi-step load test (flows, VUs, extraction, PoW) and exit.
  version            Print the version.
  help               Show this help.

attack / run flags:
  --config <path>    Path to a YAML test config (required).
  --out <path>       Write machine-readable JSON results to this file.
  --quiet            Suppress the human-readable summary.

Examples:
  gnat serve
  gnat attack --config ./loadtest.yaml --out results.json
  gnat run --config ./tooncache.yaml --out results.json
`)
}

func runServe() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c := di.New(ctx)

	addr := fmt.Sprintf(":%d", c.GetConfig().Application.Port)
	printWelcome(addr)

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
