package sandbox

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// Type is the sandbox backend type.
type Type string

const (
	Docker Type = "docker"
	Podman Type = "podman"
	None   Type = "none"
)

// Result holds sandbox execution output.
type Result struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Duration  string `json:"duration"`
	Sandboxed bool   `json:"sandboxed"`
	Error     string `json:"error,omitempty"`
}

// Backend defines the sandbox execution backend.
type Backend interface {
	Run(ctx context.Context, cmd string, workDir string, timeout time.Duration) (*Result, error)
	Available() bool
	Name() string
}

// Runner executes commands in a sandboxed environment.
type Runner struct {
	backend Backend
	timeout time.Duration
	mu      sync.Mutex
}

// NewRunner creates a sandbox runner with the best available backend.
// Tries Docker first; falls back to local execution with a warning.
func NewRunner() *Runner {
	r := &Runner{timeout: 300 * time.Second}
	docker := NewDockerBackend()
	if docker.Available() {
		r.backend = docker
	} else {
		r.backend = NewLocalBackend()
		fmt.Fprintf(os.Stderr, "[sandbox] Docker not available — using local fallback\n")
	}
	return r
}

// Run executes a command in the sandbox.
func (r *Runner) Run(ctx context.Context, cmd string, workDir string) (*Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	start := time.Now()
	result, err := r.backend.Run(ctx, cmd, workDir, r.timeout)
	if result != nil {
		result.Duration = time.Since(start).String()
	}
	return result, err
}

// SetTimeout overrides the default timeout for subsequent runs.
func (r *Runner) SetTimeout(d time.Duration) {
	r.timeout = d
}

// Backend returns the current backend name (docker, podman, local, or none).
func (r *Runner) Backend() string {
	if r.backend == nil {
		return "none"
	}
	return r.backend.Name()
}

// Stop performs any necessary cleanup (no-op for current backends).
func (r *Runner) Stop() error {
	return nil
}