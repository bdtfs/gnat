package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// DeployConfig holds options for binary deployment.
type DeployConfig struct {
	// LocalBinaryPath is the path to the local gnat binary to deploy.
	LocalBinaryPath string
	// RemotePath is the destination path on the remote host.
	RemotePath string
	// VerifyTimeout is the timeout for the version verification command.
	// Defaults to 10 seconds if zero.
	VerifyTimeout time.Duration
}

// Deploy copies the gnat binary to a remote host using SCP over an SSH session,
// sets executable permissions, and verifies the deployment by running `gnat --version`.
func Deploy(client *SSHClient, config DeployConfig) error {
	if client == nil {
		return fmt.Errorf("client is required")
	}
	if config.LocalBinaryPath == "" {
		return fmt.Errorf("local binary path is required")
	}
	if config.RemotePath == "" {
		return fmt.Errorf("remote path is required")
	}
	if config.VerifyTimeout == 0 {
		config.VerifyTimeout = 10 * time.Second
	}

	localFile, err := os.Open(config.LocalBinaryPath)
	if err != nil {
		return fmt.Errorf("opening local binary %s: %w", config.LocalBinaryPath, err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("stating local binary: %w", err)
	}

	if err := scpUpload(client.conn, localFile, stat.Size(), config.RemotePath); err != nil {
		return fmt.Errorf("uploading binary: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.VerifyTimeout)
	defer cancel()

	stdout, stderr, exitCode, err := client.Run(ctx, "chmod +x "+config.RemotePath)
	if err != nil {
		return fmt.Errorf("setting executable permissions: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("chmod failed (exit %d): %s", exitCode, stderr)
	}

	stdout, stderr, exitCode, err = client.Run(ctx, config.RemotePath+" --version")
	if err != nil {
		return fmt.Errorf("verifying deployment: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("version check failed (exit %d): %s", exitCode, stderr)
	}

	_ = stdout
	return nil
}

// DeployWithRunner deploys using the CommandRunner and FileTransferrer interfaces
// for testability.
func DeployWithRunner(runner CommandRunner, transferrer FileTransferrer, config DeployConfig) error {
	if runner == nil {
		return fmt.Errorf("runner is required")
	}
	if transferrer == nil {
		return fmt.Errorf("transferrer is required")
	}
	if config.LocalBinaryPath == "" {
		return fmt.Errorf("local binary path is required")
	}
	if config.RemotePath == "" {
		return fmt.Errorf("remote path is required")
	}
	if config.VerifyTimeout == 0 {
		config.VerifyTimeout = 10 * time.Second
	}

	data, err := os.ReadFile(config.LocalBinaryPath)
	if err != nil {
		return fmt.Errorf("reading local binary %s: %w", config.LocalBinaryPath, err)
	}

	if err := transferrer.WriteFile(config.RemotePath, data, 0755); err != nil {
		return fmt.Errorf("uploading binary: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.VerifyTimeout)
	defer cancel()

	_, stderr, exitCode, err := runner.Run(ctx, "chmod +x "+config.RemotePath)
	if err != nil {
		return fmt.Errorf("setting executable permissions: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("chmod failed (exit %d): %s", exitCode, stderr)
	}

	_, stderr, exitCode, err = runner.Run(ctx, config.RemotePath+" --version")
	if err != nil {
		return fmt.Errorf("verifying deployment: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("version check failed (exit %d): %s", exitCode, stderr)
	}

	return nil
}

// scpUpload copies a file to the remote host using the SCP protocol over SSH.
func scpUpload(conn *ssh.Client, reader io.Reader, size int64, remotePath string) error {
	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	defer session.Close()

	remoteDir := path.Dir(remotePath)
	remoteFile := path.Base(remotePath)

	pipe, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %w", err)
	}

	var stderrBuf strings.Builder
	session.Stderr = &stderrBuf

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Run("scp -t " + remoteDir)
	}()

	// Send the SCP protocol header: file mode, size, and filename
	_, err = fmt.Fprintf(pipe, "C0755 %d %s\n", size, remoteFile)
	if err != nil {
		return fmt.Errorf("writing SCP header: %w", err)
	}

	_, err = io.Copy(pipe, reader)
	if err != nil {
		return fmt.Errorf("copying file data: %w", err)
	}

	// Send the SCP termination byte
	_, err = fmt.Fprint(pipe, "\x00")
	if err != nil {
		return fmt.Errorf("writing SCP termination: %w", err)
	}

	pipe.Close()

	if err := <-errCh; err != nil {
		return fmt.Errorf("scp command failed: %s: %w", stderrBuf.String(), err)
	}

	return nil
}
