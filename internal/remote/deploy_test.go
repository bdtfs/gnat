package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDeploy_NilClient(t *testing.T) {
	t.Parallel()

	err := Deploy(nil, DeployConfig{
		LocalBinaryPath: "/tmp/gnat",
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
	if !containsString(err.Error(), "client is required") {
		t.Errorf("expected 'client is required' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_NilRunner(t *testing.T) {
	t.Parallel()

	err := DeployWithRunner(nil, newMockTransferrer(), DeployConfig{
		LocalBinaryPath: "/tmp/gnat",
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nil runner, got nil")
	}
	if !containsString(err.Error(), "runner is required") {
		t.Errorf("expected 'runner is required' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_NilTransferrer(t *testing.T) {
	t.Parallel()

	err := DeployWithRunner(newMockRunner(), nil, DeployConfig{
		LocalBinaryPath: "/tmp/gnat",
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nil transferrer, got nil")
	}
	if !containsString(err.Error(), "transferrer is required") {
		t.Errorf("expected 'transferrer is required' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_EmptyLocalPath(t *testing.T) {
	t.Parallel()

	err := DeployWithRunner(newMockRunner(), newMockTransferrer(), DeployConfig{
		RemotePath: "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for empty local path, got nil")
	}
	if !containsString(err.Error(), "local binary path is required") {
		t.Errorf("expected 'local binary path is required' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_EmptyRemotePath(t *testing.T) {
	t.Parallel()

	err := DeployWithRunner(newMockRunner(), newMockTransferrer(), DeployConfig{
		LocalBinaryPath: "/tmp/gnat",
	})

	if err == nil {
		t.Fatal("expected error for empty remote path, got nil")
	}
	if !containsString(err.Error(), "remote path is required") {
		t.Errorf("expected 'remote path is required' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_NonexistentLocalBinary(t *testing.T) {
	t.Parallel()

	err := DeployWithRunner(newMockRunner(), newMockTransferrer(), DeployConfig{
		LocalBinaryPath: "/nonexistent/binary/gnat",
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for nonexistent local binary, got nil")
	}
}

func TestDeployWithRunner_TransferFailure(t *testing.T) {
	t.Parallel()

	tmpFile := createTempBinary(t)
	transfer := newMockTransferrer()
	transfer.shouldFail = true

	err := DeployWithRunner(newMockRunner(), transfer, DeployConfig{
		LocalBinaryPath: tmpFile,
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for transfer failure, got nil")
	}
	if !containsString(err.Error(), "uploading binary") {
		t.Errorf("expected 'uploading binary' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_ChmodFailure(t *testing.T) {
	t.Parallel()

	tmpFile := createTempBinary(t)
	runner := newMockRunner()
	runner.onCommand("chmod +x /usr/local/bin/gnat", mockResponse{
		stderr:   "permission denied",
		exitCode: 1,
	})

	err := DeployWithRunner(runner, newMockTransferrer(), DeployConfig{
		LocalBinaryPath: tmpFile,
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for chmod failure, got nil")
	}
	if !containsString(err.Error(), "chmod failed") {
		t.Errorf("expected 'chmod failed' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_VersionCheckFailure(t *testing.T) {
	t.Parallel()

	tmpFile := createTempBinary(t)
	runner := newMockRunner()
	runner.onCommand("chmod +x /usr/local/bin/gnat", mockResponse{exitCode: 0})
	runner.onCommand("/usr/local/bin/gnat --version", mockResponse{
		stderr:   "command not found",
		exitCode: 127,
	})

	err := DeployWithRunner(runner, newMockTransferrer(), DeployConfig{
		LocalBinaryPath: tmpFile,
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error for version check failure, got nil")
	}
	if !containsString(err.Error(), "version check failed") {
		t.Errorf("expected 'version check failed' error, got %q", err.Error())
	}
}

func TestDeployWithRunner_Success(t *testing.T) {
	t.Parallel()

	tmpFile := createTempBinary(t)
	runner := newMockRunner()
	runner.onCommand("chmod +x /usr/local/bin/gnat", mockResponse{exitCode: 0})
	runner.onCommand("/usr/local/bin/gnat --version", mockResponse{
		stdout:   "gnat version 0.1.0",
		exitCode: 0,
	})

	transfer := newMockTransferrer()

	err := DeployWithRunner(runner, transfer, DeployConfig{
		LocalBinaryPath: tmpFile,
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(runner.commands))
	}
	if runner.commands[0] != "chmod +x /usr/local/bin/gnat" {
		t.Errorf("expected chmod command, got %q", runner.commands[0])
	}
	if runner.commands[1] != "/usr/local/bin/gnat --version" {
		t.Errorf("expected version command, got %q", runner.commands[1])
	}

	if _, ok := transfer.files["/usr/local/bin/gnat"]; !ok {
		t.Error("expected file to be uploaded")
	}
}

func TestDeployWithRunner_TransferError(t *testing.T) {
	t.Parallel()

	tmpFile := createTempBinary(t)
	transfer := newMockTransferrer()
	transfer.shouldFail = true
	transfer.failErr = fmt.Errorf("disk full")

	err := DeployWithRunner(newMockRunner(), transfer, DeployConfig{
		LocalBinaryPath: tmpFile,
		RemotePath:      "/usr/local/bin/gnat",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsString(err.Error(), "disk full") {
		t.Errorf("expected error containing 'disk full', got %q", err.Error())
	}
}

func TestDeploy_RealSSH(t *testing.T) {
	t.Skip("requires SSH server")

	// This test would deploy to a real remote server
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

	err = Deploy(client, DeployConfig{
		LocalBinaryPath: "/usr/local/bin/gnat",
		RemotePath:      "/tmp/gnat",
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
}

// createTempBinary creates a temporary file to act as a binary for testing.
func createTempBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gnat")
	if err := os.WriteFile(path, []byte("fake binary content"), 0755); err != nil {
		t.Fatalf("failed to create temp binary: %v", err)
	}
	return path
}
