package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DockerSandbox runs commands inside Docker containers for real isolation.
type DockerSandbox struct {
	Image     string        // container image (default: alpine:latest)
	Memory    string        // memory limit (default: 512m)
	CPUS      string        // CPU limit (default: 1)
	Network   string        // network mode (default: none)
	Timeout   time.Duration // max execution time (default: 300s)
	available bool          // cached availability check
}

// NewDockerSandbox creates a Docker sandbox. Checks Docker availability on creation.
func NewDockerSandbox() *DockerSandbox {
	ds := &DockerSandbox{
		Image:   "alpine:latest",
		Memory:  "512m",
		CPUS:    "1",
		Network: "none",
		Timeout: 300 * time.Second,
	}
	ds.available = ds.checkAvailable()
	return ds
}

// IsAvailable returns true if Docker is usable.
func (ds *DockerSandbox) IsAvailable() bool { return ds.available }

func (ds *DockerSandbox) checkAvailable() bool {
	cmd := exec.Command("docker", "info")
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// RunCommand executes a command inside a temporary Docker container.
func (ds *DockerSandbox) RunCommand(ctx context.Context, workDir string, command string) (string, string, int, error) {
	if !ds.available {
		return "", "", -1, fmt.Errorf("docker not available")
	}

	if ds.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ds.Timeout)
		defer cancel()
	}

	containerName := fmt.Sprintf("tau-sandbox-%d", time.Now().UnixNano())

	createArgs := []string{
		"run", "--rm",
		"--name", containerName,
		"--network", ds.Network,
		"--memory", ds.Memory,
		"--cpus", ds.CPUS,
		"--read-only",
		"--tmpfs", "/tmp:exec",
		"-w", "/workspace",
		"-d",
		ds.Image,
		"tail", "-f", "/dev/null",
	}

	if workDir != "" {
		createArgs = append(createArgs, "-v", workDir+":/workspace:ro")
	}

	createCmd := exec.Command("docker", createArgs...)
	if out, err := createCmd.CombinedOutput(); err != nil {
		return "", string(out), -1, fmt.Errorf("docker create: %w (output: %s)", err, out)
	}

	defer func() {
		exec.Command("docker", "rm", "-f", containerName).Run()
	}()

	execArgs := []string{"exec", containerName, "sh", "-c", command}
	execCmd := exec.CommandContext(ctx, "docker", execArgs...)

	var stdout, stderr strings.Builder
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

	return stdout.String(), stderr.String(), exitCode, err
}

// WrapCommand wraps an exec.Cmd to run inside Docker. No-op if Docker unavailable.
func (ds *DockerSandbox) WrapCommand(cmd *exec.Cmd) {
	if !ds.available {
		return
	}

	containerName := fmt.Sprintf("tau-sandbox-%d", time.Now().UnixNano())
	workDir := cmd.Dir

	createArgs := []string{
		"run", "--rm", "-d",
		"--name", containerName,
		"--network", "none",
		"--memory", "512m", "--cpus", "1",
		"--read-only", "--tmpfs", "/tmp:exec",
		"-w", workDir,
		ds.Image, "tail", "-f", "/dev/null",
	}
	if workDir != "" {
		createArgs = append(createArgs, "-v", workDir+":/workspace:ro")
	}

	createCmd := exec.Command("docker", createArgs...)
	if err := createCmd.Run(); err != nil {
		return // silent fallback
	}

	originalArgs := cmd.Args
	newArgs := append([]string{"exec", containerName}, originalArgs...)
	cmd.Args = newArgs
	cmd.Path = "docker"

	go func() {
		cmd.Wait()
		exec.Command("docker", "rm", "-f", containerName).Run()
	}()
}
