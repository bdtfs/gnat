package remote

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// FetchResultsConfig configures result fetching from a remote host.
type FetchResultsConfig struct {
	// BinaryPath is the path to the gnat binary on the remote host.
	BinaryPath string
	// Timeout is the maximum duration for fetching results.
	// Defaults to 30 seconds if zero.
	Timeout time.Duration
}

// FetchResults retrieves the result JSON for a given run ID from the remote host.
// It calls `gnat run status <runID> --output json` on the remote to get the results.
func FetchResults(client *SSHClient, runID string, config FetchResultsConfig) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}

	return fetchResultsWithRunner(client, runID, config)
}

// FetchResultsWithRunner retrieves results using the CommandRunner interface
// for testability.
func FetchResultsWithRunner(runner CommandRunner, runID string, config FetchResultsConfig) ([]byte, error) {
	if runner == nil {
		return nil, fmt.Errorf("runner is required")
	}

	return fetchResultsWithRunner(runner, runID, config)
}

func fetchResultsWithRunner(runner CommandRunner, runID string, config FetchResultsConfig) ([]byte, error) {
	if runID == "" {
		return nil, fmt.Errorf("run ID is required")
	}
	if config.BinaryPath == "" {
		return nil, fmt.Errorf("binary path is required")
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	cmd := fmt.Sprintf("%s run status %s --output json", config.BinaryPath, runID)

	stdout, stderr, exitCode, err := runner.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("fetching results: %w", err)
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("fetch results failed (exit %d): %s", exitCode, stderr)
	}

	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		return nil, fmt.Errorf("empty result output for run %s", runID)
	}

	return []byte(trimmed), nil
}
