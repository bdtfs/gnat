package remote

import (
	"context"
	"fmt"
	"os"
)

// mockRunner implements CommandRunner for testing.
type mockRunner struct {
	// commands records all commands that were run.
	commands []string
	// responses maps command strings to mock responses.
	responses map[string]mockResponse
	// defaultResponse is returned when no specific response is configured.
	defaultResponse mockResponse
}

type mockResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		responses: make(map[string]mockResponse),
	}
}

func (m *mockRunner) Run(_ context.Context, cmd string) (string, string, int, error) {
	m.commands = append(m.commands, cmd)
	if resp, ok := m.responses[cmd]; ok {
		return resp.stdout, resp.stderr, resp.exitCode, resp.err
	}
	return m.defaultResponse.stdout, m.defaultResponse.stderr, m.defaultResponse.exitCode, m.defaultResponse.err
}

func (m *mockRunner) Close() error {
	return nil
}

func (m *mockRunner) onCommand(cmd string, resp mockResponse) {
	m.responses[cmd] = resp
}

// mockTransferrer implements FileTransferrer for testing.
type mockTransferrer struct {
	files      map[string][]byte
	perms      map[string]os.FileMode
	shouldFail bool
	failErr    error
}

func newMockTransferrer() *mockTransferrer {
	return &mockTransferrer{
		files: make(map[string][]byte),
		perms: make(map[string]os.FileMode),
	}
}

func (m *mockTransferrer) WriteFile(remotePath string, data []byte, perm os.FileMode) error {
	if m.shouldFail {
		if m.failErr != nil {
			return m.failErr
		}
		return fmt.Errorf("mock transfer error")
	}
	m.files[remotePath] = data
	m.perms[remotePath] = perm
	return nil
}
