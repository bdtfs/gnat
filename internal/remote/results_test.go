package remote

import (
	"fmt"
	"testing"
)

func TestFetchResults_NilClient(t *testing.T) {
	t.Parallel()

	_, err := FetchResults(nil, "run-123", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
	if !containsString(err.Error(), "client is required") {
		t.Errorf("expected 'client is required' error, got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_NilRunner(t *testing.T) {
	t.Parallel()

	_, err := FetchResultsWithRunner(nil, "run-123", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nil runner, got nil")
	}
	if !containsString(err.Error(), "runner is required") {
		t.Errorf("expected 'runner is required' error, got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_EmptyRunID(t *testing.T) {
	t.Parallel()

	_, err := FetchResultsWithRunner(newMockRunner(), "", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for empty run ID, got nil")
	}
	if !containsString(err.Error(), "run ID is required") {
		t.Errorf("expected 'run ID is required' error, got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_EmptyBinaryPath(t *testing.T) {
	t.Parallel()

	_, err := FetchResultsWithRunner(newMockRunner(), "run-123", FetchResultsConfig{})

	if err == nil {
		t.Fatal("expected error for empty binary path, got nil")
	}
	if !containsString(err.Error(), "binary path is required") {
		t.Errorf("expected 'binary path is required' error, got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_Success(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	expectedJSON := `{"status":"completed","total":1000,"success":990}`
	runner.onCommand("/usr/local/bin/gnat run status run-123 --output json", mockResponse{
		stdout:   expectedJSON,
		exitCode: 0,
	})

	result, err := FetchResultsWithRunner(runner, "run-123", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != expectedJSON {
		t.Errorf("expected %q, got %q", expectedJSON, string(result))
	}
}

func TestFetchResultsWithRunner_NonZeroExitCode(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run status run-404 --output json", mockResponse{
		stderr:   "run not found",
		exitCode: 1,
	})

	_, err := FetchResultsWithRunner(runner, "run-404", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for non-zero exit code, got nil")
	}
	if !containsString(err.Error(), "fetch results failed") {
		t.Errorf("expected 'fetch results failed' error, got %q", err.Error())
	}
	if !containsString(err.Error(), "run not found") {
		t.Errorf("expected error containing stderr, got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_EmptyOutput(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run status run-123 --output json", mockResponse{
		stdout:   "",
		exitCode: 0,
	})

	_, err := FetchResultsWithRunner(runner, "run-123", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for empty output, got nil")
	}
	if !containsString(err.Error(), "empty result output") {
		t.Errorf("expected 'empty result output' error, got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_WhitespaceOnlyOutput(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run status run-123 --output json", mockResponse{
		stdout:   "   \n\t  \n",
		exitCode: 0,
	})

	_, err := FetchResultsWithRunner(runner, "run-123", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for whitespace-only output, got nil")
	}
	if !containsString(err.Error(), "empty result output") {
		t.Errorf("expected 'empty result output' error, got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_RunnerError(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.onCommand("/usr/local/bin/gnat run status run-123 --output json", mockResponse{
		err: fmt.Errorf("connection lost"),
	})

	_, err := FetchResultsWithRunner(runner, "run-123", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for runner failure, got nil")
	}
	if !containsString(err.Error(), "connection lost") {
		t.Errorf("expected error containing 'connection lost', got %q", err.Error())
	}
}

func TestFetchResultsWithRunner_TrimmedOutput(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	expectedJSON := `{"status":"completed"}`
	runner.onCommand("/usr/local/bin/gnat run status run-123 --output json", mockResponse{
		stdout:   "\n  " + expectedJSON + "  \n",
		exitCode: 0,
	})

	result, err := FetchResultsWithRunner(runner, "run-123", FetchResultsConfig{
		BinaryPath: "/usr/local/bin/gnat",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != expectedJSON {
		t.Errorf("expected trimmed output %q, got %q", expectedJSON, string(result))
	}
}

func TestFetchResultsWithRunner_CorrectCommandFormat(t *testing.T) {
	t.Parallel()

	runner := newMockRunner()
	runner.defaultResponse = mockResponse{
		stdout:   `{"status":"completed"}`,
		exitCode: 0,
	}

	_, err := FetchResultsWithRunner(runner, "abc-def-123", FetchResultsConfig{
		BinaryPath: "/opt/gnat",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.commands))
	}

	expected := "/opt/gnat run status abc-def-123 --output json"
	if runner.commands[0] != expected {
		t.Errorf("expected command %q, got %q", expected, runner.commands[0])
	}
}

func TestFetchResults_RealSSH(t *testing.T) {
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

	result, err := FetchResults(client, "test-run-id", FetchResultsConfig{
		BinaryPath: "/tmp/gnat",
	})
	if err != nil {
		t.Fatalf("fetch results failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}
