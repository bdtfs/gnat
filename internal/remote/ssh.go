package remote

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHConfig holds the configuration for establishing an SSH connection.
type SSHConfig struct {
	Host           string
	Port           int
	User           string
	KeyPath        string
	KeyPassphrase  string
	KnownHostsPath string
	Timeout        time.Duration
}

// CommandRunner is the interface for executing remote commands.
// This abstraction enables testing without a real SSH connection.
type CommandRunner interface {
	Run(ctx context.Context, cmd string) (stdout string, stderr string, exitCode int, err error)
	Close() error
}

// FileTransferrer is the interface for transferring files to a remote host.
type FileTransferrer interface {
	WriteFile(remotePath string, data []byte, perm os.FileMode) error
}

// SSHClient wraps an SSH connection and provides methods for remote operations.
type SSHClient struct {
	conn   *ssh.Client
	config SSHConfig
}

// NewSSHClient establishes an SSH connection using the provided configuration.
// It supports key-based auth (from file path) and SSH agent auth.
func NewSSHClient(config SSHConfig) (*SSHClient, error) {
	if config.Host == "" {
		return nil, fmt.Errorf("host is required")
	}
	if config.User == "" {
		return nil, fmt.Errorf("user is required")
	}
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	authMethods, err := buildAuthMethods(config)
	if err != nil {
		return nil, fmt.Errorf("building auth methods: %w", err)
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available: provide key path or SSH agent")
	}

	hostKeyCallback, err := buildHostKeyCallback(config.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("building host key callback: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         config.Timeout,
	}

	addr := net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port))

	conn, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", addr, err)
	}

	return &SSHClient{
		conn:   conn,
		config: config,
	}, nil
}

// Run executes a command on the remote host and returns stdout, stderr, exit code, and any error.
// The command is cancelled if the context is done.
func (c *SSHClient) Run(ctx context.Context, cmd string) (stdout string, stderr string, exitCode int, err error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", "", -1, fmt.Errorf("creating session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf strings.Builder
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return stdoutBuf.String(), stderrBuf.String(), -1, ctx.Err()
	case runErr := <-done:
		if runErr != nil {
			if exitErr, ok := runErr.(*ssh.ExitError); ok {
				return stdoutBuf.String(), stderrBuf.String(), exitErr.ExitStatus(), nil
			}
			return stdoutBuf.String(), stderrBuf.String(), -1, fmt.Errorf("running command: %w", runErr)
		}
		return stdoutBuf.String(), stderrBuf.String(), 0, nil
	}
}

// Close closes the underlying SSH connection.
func (c *SSHClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Conn returns the underlying ssh.Client for advanced operations like SFTP.
func (c *SSHClient) Conn() *ssh.Client {
	return c.conn
}

// buildAuthMethods constructs SSH authentication methods from the config.
// It attempts key-based auth first, then falls back to SSH agent.
func buildAuthMethods(config SSHConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if config.KeyPath != "" {
		keyAuth, err := keyFileAuth(config.KeyPath, config.KeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("reading key file %s: %w", config.KeyPath, err)
		}
		methods = append(methods, keyAuth)
	}

	agentAuth, err := sshAgentAuth()
	if err == nil && agentAuth != nil {
		methods = append(methods, agentAuth)
	}

	return methods, nil
}

// keyFileAuth reads a private key file and returns an ssh.AuthMethod.
func keyFileAuth(keyPath, passphrase string) (ssh.AuthMethod, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}

	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}

	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

// sshAgentAuth attempts to connect to the SSH agent and returns an auth method.
func sshAgentAuth() (ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("connecting to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// buildHostKeyCallback constructs a host key callback.
// If a known_hosts path is provided, it uses strict host key checking.
// Otherwise, it uses InsecureIgnoreHostKey (suitable for development).
func buildHostKeyCallback(knownHostsPath string) (ssh.HostKeyCallback, error) {
	if knownHostsPath == "" {
		//nolint:gosec // InsecureIgnoreHostKey is intentional when no known_hosts is provided
		return ssh.InsecureIgnoreHostKey(), nil
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("parsing known_hosts file %s: %w", knownHostsPath, err)
	}

	return callback, nil
}
