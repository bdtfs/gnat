package remote

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRemoteRun_NilClient(t *testing.T) {
	t.Parallel()

	_, err := RemoteRun(nil, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
	if !containsString(err.Error(), "client is required") {
		t.Errorf("expected 'client is required' error, got %q", err.Error())
	}
}

func TestRemoteRunWithRunner_NilRunner(t *testing.T) {
	t.Parallel()

	_, err := RemoteRunWithRunner(nil, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nil runner, got nil")
	}
	if !containsString(err.Error(), "runner is required") {
		t.Errorf("expected 'runner is required' error, got %q", err.Error())
	}
}

func TestRemoteRunWithRunner_EmptyBinaryPath(t *testing.T) {
	t.Parallel()

	_, err := RemoteRunWithRunner(newMockRunner(), context.Background(), RemoteRunConfig{})

	if err == nil {
		t.Fatal("expected error for empty binary path, got nil")
	}
	if !containsString(err.Error(), "binary path is required") {
		t.Errorf("expected 'binary path is required' error, got %q", err.Error())
	}
}

func TestRemoteRunWithRunner_Success_JSONOutput(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	jsonOutput := `{"status":"completed","total":1000,"success":990}`
	runner.onCommand("/usr/local/bin/gnat run --url https://example.com --rps 100 --duration 30s", mockResponse{
		stdout:   jsonOutput,
		exitCode: 0,
	})

	result, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
		Args:       []string{"--url", "https://example.com", "--rps", "100", "--duration", "30s"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != jsonOutput {
		t.Errorf("expected stdout %q, got %q", jsonOutput, result.Stdout)
	}
	if result.JSONOutput == nil {
		t.Fatal("expected non-nil JSONOutput")
	}
	if string(result.JSONOutput) != jsonOutput {
		t.Errorf("expected JSONOutput %q, got %q", jsonOutput, string(result.JSONOutput))
	}
}

func TestRemoteRunWithRunner_Success_NonJSONOutput(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run --url https://example.com", mockResponse{
		stdout:   "Test completed: 1000 requests",
		exitCode: 0,
	})

	result, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
		Args:       []string{"--url", "https://example.com"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.JSONOutput != nil {
		t.Errorf("expected nil JSONOutput for non-JSON output, got %q", string(result.JSONOutput))
	}
}

func TestRemoteRunWithRunner_NonZeroExitCode(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run --url https://example.com", mockResponse{
		stderr:   "connection refused",
		exitCode: 3,
	})

	result, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
		Args:       []string{"--url", "https://example.com"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 3 {
		t.Errorf("expected exit code 3, got %d", result.ExitCode)
	}
	if result.Stderr != "connection refused" {
		t.Errorf("expected stderr 'connection refused', got %q", result.Stderr)
	}
}

func TestRemoteRunWithRunner_RunnerError(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run --url https://example.com", mockResponse{
		err: fmt.Errorf("network timeout"),
	})

	_, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
		Args:       []string{"--url", "https://example.com"},
	})

	if err == nil {
		t.Fatal("expected error for runner failure, got nil")
	}
	if !containsString(err.Error(), "network timeout") {
		t.Errorf("expected error containing 'network timeout', got %q", err.Error())
	}
}

func TestRemoteRunWithRunner_NoArgs(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run", mockResponse{
		stdout:   `{"status":"completed"}`,
		exitCode: 0,
	})

	result, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestRemoteRunWithRunner_DefaultTimeout(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.defaultResponse = mockResponse{exitCode: 0}

	_, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
		Timeout:    0, // should default to 10 minutes
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoteRunWithRunner_CustomTimeout(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.defaultResponse = mockResponse{exitCode: 0}

	_, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
		Timeout:    5 * time.Minute,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoteRunWithRunner_ContextCancellation(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.defaultResponse = mockResponse{
		err: context.Canceled,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RemoteRunWithRunner(runner, ctx, RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestRemoteRunWithRunner_StderrCapture(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run --url https://example.com", mockResponse{
		stdout:   `{"status":"completed"}`,
		stderr:   "warning: high latency detected",
		exitCode: 0,
	})

	result, err := RemoteRunWithRunner(runner, context.Background(), RemoteRunConfig{
		BinaryPath: "/usr/local/bin/gnat",
		Args:       []string{"--url", "https://example.com"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stderr != "warning: high latency detected" {
		t.Errorf("expected stderr capture, got %q", result.Stderr)
	}
}

func TestBuildCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		binaryPath string
		args       []string
		expected   string
	}{
		{
			name:       "no args",
			binaryPath: "/usr/local/bin/gnat",
			args:       nil,
			expected:   "/usr/local/bin/gnat run",
		},
		{
			name:       "with args",
			binaryPath: "/usr/local/bin/gnat",
			args:       []string{"--url", "https://example.com", "--rps", "100"},
			expected:   "/usr/local/bin/gnat run --url https://example.com --rps 100",
		},
		{
			name:       "single arg",
			binaryPath: "/opt/gnat",
			args:       []string{"--config", "test.yaml"},
			expected:   "/opt/gnat run --config test.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildCommand(tt.binaryPath, tt.args)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestRemoteResult_Fields(t *testing.T) {
	t.Parallel()

	result := RemoteResult{
		Stdout:   "test output",
		Stderr:   "test error",
		ExitCode: 42,
	}

	if result.Stdout != "test output" {
		t.Errorf("expected Stdout 'test output', got %q", result.Stdout)
	}
	if result.Stderr != "test error" {
		t.Errorf("expected Stderr 'test error', got %q", result.Stderr)
	}
	if result.ExitCode != 42 {
		t.Errorf("expected ExitCode 42, got %d", result.ExitCode)
	}
	if result.JSONOutput != nil {
		t.Errorf("expected nil JSONOutput, got %v", result.JSONOutput)
	}
}

func TestRemoteRun_RealSSH(t *testing.T) {
	t.Skip("requires SSH server")

	client, err := NewSSHClient(SSHConfig{
		Host:    "localhost",
		Port:    22,
		User:    "testuser",
		KeyPath: "~/.ssh/id_rsa",
	})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	result, err := RemoteRun(client, context.Background(), RemoteRunConfig{
		BinaryPath: "/tmp/gnat",
		Args:       []string{"--url", "https://example.com", "--rps", "1", "--duration", "1s", "--output", "json"},
	})
	if err != nil {
		t.Fatalf("remote run failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}
