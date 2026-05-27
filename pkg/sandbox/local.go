package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// LocalBackend runs commands locally without container isolation.
// This is the fallback when Docker is unavailable.
type LocalBackend struct{}

// NewLocalBackend creates a local execution backend.
func NewLocalBackend() *LocalBackend { return &LocalBackend{} }

// Available always returns true (local execution is always available).
func (l *LocalBackend) Available() bool { return true }

// Name returns the backend identifier.
func (l *LocalBackend) Name() string { return "local" }

// Run executes a command directly on the host.
// Emits a warning on stderr about the lack of isolation.
func (l *LocalBackend) Run(ctx context.Context, cmdStr string, workDir string, timeout time.Duration) (*Result, error) {
	fmt.Fprintf(os.Stderr, "[sandbox] WARNING: running locally without isolation\n")

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &Result{
		Stdout:    truncateOutput(stdout.String()),
		Stderr:    truncateOutput(stderr.String()),
		ExitCode:  exitCode,
		Sandboxed: false,
	}, nil
}