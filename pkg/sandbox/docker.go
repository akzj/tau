package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DockerBackend runs commands in Docker containers.
type DockerBackend struct {
	image   string
	network string
	memory  string
	cpus    string
}

// NewDockerBackend creates a DockerBackend with safe defaults.
func NewDockerBackend() *DockerBackend {
	return &DockerBackend{
		image:   "alpine:latest",
		network: "none",
		memory:  "512m",
		cpus:    "1",
	}
}

// Available returns true if Docker is installed and reachable.
func (d *DockerBackend) Available() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// Name returns the backend identifier.
func (d *DockerBackend) Name() string { return "docker" }

// Run executes a command inside a temporary Docker container.
// The container is created with --read-only rootfs, tmpfs /tmp,
// --network=none isolation, and CPU/memory limits.
func (d *DockerBackend) Run(ctx context.Context, cmdStr string, workDir string, timeout time.Duration) (*Result, error) {
	if !d.Available() {
		return &Result{ExitCode: -1, Error: "docker not available"}, fmt.Errorf("docker not available")
	}

	containerName := fmt.Sprintf("tau-sandbox-%d", time.Now().UnixNano())

	// Build create args
	createArgs := []string{
		"run", "--rm", "-d",
		"--name", containerName,
		"--network", d.network,
		"--memory", d.memory,
		"--cpus", d.cpus,
		"--read-only",
		"--tmpfs", "/tmp:exec,size=256m",
		"-w", "/workspace",
	}
	if workDir != "" {
		createArgs = append(createArgs, "-v", workDir+":/workspace:ro")
	}
	createArgs = append(createArgs, d.image, "tail", "-f", "/dev/null")

	// Create container
	createCmd := exec.Command("docker", createArgs...)
	if out, err := createCmd.CombinedOutput(); err != nil {
		return &Result{ExitCode: -1, Error: fmt.Sprintf("docker create: %s", string(out))}, err
	}

	// Always clean up
	defer func() {
		exec.Command("docker", "rm", "-f", containerName).Run()
	}()

	// Execute command inside container
	execArgs := []string{"exec", containerName, "sh", "-c", cmdStr}
	execCmd := exec.CommandContext(ctx, "docker", execArgs...)

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()
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
		Sandboxed: true,
	}, nil
}

// truncateOutput caps output at 4000 chars (one API response).
func truncateOutput(s string) string {
	if len(s) > 4000 {
		return s[:4000] + "\n... (truncated)"
	}
	return s
}