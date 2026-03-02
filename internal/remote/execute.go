package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RemoteResult holds the outcome of a remote gnat run execution.
type RemoteResult struct {
	// Stdout is the raw standard output from the remote command.
	Stdout string
	// Stderr is the raw standard error from the remote command.
	Stderr string
	// ExitCode is the exit code of the remote command.
	ExitCode int
	// JSONOutput is the parsed JSON output (nil if output is not valid JSON).
	JSONOutput json.RawMessage
}

// RemoteRunConfig configures a remote test execution.
type RemoteRunConfig struct {
	// BinaryPath is the path to the gnat binary on the remote host.
	BinaryPath string
	// Args are the arguments to pass to `gnat run`.
	Args []string
	// Timeout is the maximum duration for the remote execution.
	// Defaults to 10 minutes if zero.
	Timeout time.Duration
}

// RemoteRun executes `gnat run` on the remote host with the provided arguments,
// captures and returns the JSON output. It handles timeouts and cancellation via context.
func RemoteRun(client *SSHClient, ctx context.Context, config RemoteRunConfig) (*RemoteResult, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}

	return remoteRunWithRunner(client, ctx, config)
}

// RemoteRunWithRunner executes a remote gnat run using the CommandRunner interface
// for testability.
func RemoteRunWithRunner(runner CommandRunner, ctx context.Context, config RemoteRunConfig) (*RemoteResult, error) {
	if runner == nil {
		return nil, fmt.Errorf("runner is required")
	}

	return remoteRunWithRunner(runner, ctx, config)
}

func remoteRunWithRunner(runner CommandRunner, ctx context.Context, config RemoteRunConfig) (*RemoteResult, error) {
	if config.BinaryPath == "" {
		return nil, fmt.Errorf("binary path is required")
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Minute
	}

	cmd := buildCommand(config.BinaryPath, config.Args)

	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	stdout, stderr, exitCode, err := runner.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("executing remote command: %w", err)
	}

	result := &RemoteResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}

	// Try to parse the stdout as JSON
	trimmed := strings.TrimSpace(stdout)
	if json.Valid([]byte(trimmed)) {
		result.JSONOutput = json.RawMessage(trimmed)
	}

	return result, nil
}

// buildCommand constructs the full command string for remote execution.
func buildCommand(binaryPath string, args []string) string {
	parts := make([]string, 0, len(args)+2)
	parts = append(parts, binaryPath, "run")
	parts = append(parts, args...)
	return strings.Join(parts, " ")
}
