package remote

import (
	"testing"
	"time"
)

func TestSSHConfig_Defaults(t *testing.T) {
	t.Parallel()

	config := SSHConfig{
		Host: "example.com",
		User: "testuser",
	}

	if config.Port != 0 {
		t.Errorf("expected default Port 0, got %d", config.Port)
	}
	if config.Timeout != 0 {
		t.Errorf("expected default Timeout 0, got %v", config.Timeout)
	}
}

func TestSSHConfig_Fields(t *testing.T) {
	t.Parallel()

	config := SSHConfig{
		Host:            "example.com",
		Port:            2222,
		User:            "admin",
		KeyPath:         "/home/admin/.ssh/id_rsa",
		KeyPassphrase:   "secret",
		KnownHostsPath: "/home/admin/.ssh/known_hosts",
		Timeout:         60 * time.Second,
	}

	if config.Host != "example.com" {
		t.Errorf("expected Host 'example.com', got %q", config.Host)
	}
	if config.Port != 2222 {
		t.Errorf("expected Port 2222, got %d", config.Port)
	}
	if config.User != "admin" {
		t.Errorf("expected User 'admin', got %q", config.User)
	}
	if config.KeyPath != "/home/admin/.ssh/id_rsa" {
		t.Errorf("expected KeyPath '/home/admin/.ssh/id_rsa', got %q", config.KeyPath)
	}
	if config.KeyPassphrase != "secret" {
		t.Errorf("expected KeyPassphrase 'secret', got %q", config.KeyPassphrase)
	}
	if config.KnownHostsPath != "/home/admin/.ssh/known_hosts" {
		t.Errorf("expected KnownHostsPath, got %q", config.KnownHostsPath)
	}
	if config.Timeout != 60*time.Second {
		t.Errorf("expected Timeout 60s, got %v", config.Timeout)
	}
}

func TestNewSSHClient_MissingHost(t *testing.T) {
	t.Parallel()

	_, err := NewSSHClient(SSHConfig{
		User:    "testuser",
		KeyPath: "/tmp/nonexistent",
	})

	if err == nil {
		t.Fatal("expected error for missing host, got nil")
	}
	if !containsString(err.Error(), "host is required") {
		t.Errorf("expected error containing 'host is required', got %q", err.Error())
	}
}

func TestNewSSHClient_MissingUser(t *testing.T) {
	t.Parallel()

	_, err := NewSSHClient(SSHConfig{
		Host:    "example.com",
		KeyPath: "/tmp/nonexistent",
	})

	if err == nil {
		t.Fatal("expected error for missing user, got nil")
	}
	if !containsString(err.Error(), "user is required") {
		t.Errorf("expected error containing 'user is required', got %q", err.Error())
	}
}

func TestNewSSHClient_InvalidKeyPath(t *testing.T) {
	t.Parallel()

	_, err := NewSSHClient(SSHConfig{
		Host:    "example.com",
		User:    "testuser",
		KeyPath: "/nonexistent/path/id_rsa",
	})

	if err == nil {
		t.Fatal("expected error for invalid key path, got nil")
	}
}

func TestNewSSHClient_NoAuthMethods(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := NewSSHClient(SSHConfig{
		Host: "example.com",
		User: "testuser",
	})

	if err == nil {
		t.Fatal("expected error for no auth methods, got nil")
	}
	if !containsString(err.Error(), "no authentication methods available") {
		t.Errorf("expected error containing 'no authentication methods available', got %q", err.Error())
	}
}

func TestNewSSHClient_RealConnection(t *testing.T) {
	t.Skip("requires SSH server")

	client, err := NewSSHClient(SSHConfig{
		Host:    "localhost",
		Port:    22,
		User:    "testuser",
		KeyPath: "~/.ssh/id_rsa",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()
}

func TestBuildHostKeyCallback_InsecureWhenEmpty(t *testing.T) {
	t.Parallel()

	callback, err := buildHostKeyCallback("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callback == nil {
		t.Fatal("expected non-nil callback")
	}
}

func TestBuildHostKeyCallback_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := buildHostKeyCallback("/nonexistent/known_hosts")
	if err == nil {
		t.Fatal("expected error for invalid known_hosts path, got nil")
	}
}

func TestBuildAuthMethods_NoKeyNoAgent(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("SSH_AUTH_SOCK", "")

	methods, err := buildAuthMethods(SSHConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 0 {
		t.Errorf("expected 0 auth methods, got %d", len(methods))
	}
}

func TestBuildAuthMethods_InvalidKeyFile(t *testing.T) {
	t.Parallel()

	_, err := buildAuthMethods(SSHConfig{
		KeyPath: "/nonexistent/key",
	})

	if err == nil {
		t.Fatal("expected error for invalid key file, got nil")
	}
}

func TestSSHClient_Close_NilConn(t *testing.T) {
	t.Parallel()

	client := &SSHClient{}
	err := client.Close()
	if err != nil {
		t.Errorf("expected nil error for nil conn, got %v", err)
	}
}
