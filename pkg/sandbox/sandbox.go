package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Type is the sandbox backend.
type Type string

const (
	Docker Type = "docker"
	Podman Type = "podman"
	None   Type = "none"
)

// Result captures the output of a sandboxed command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes commands in a sandboxed environment.
type Runner interface {
	Run(ctx context.Context, cmdStr string, workDir string, timeout time.Duration) (Result, error)
	Backend() Type
}

// Detect returns the best available sandbox runner.
// Falls back to PathJail if container is unavailable.
func Detect(preferred Type, workspaceRoot string) Runner {
	if preferred == None {
		return &PathJail{root: workspaceRoot}
	}
	bin := string(preferred)
	if _, err := exec.LookPath(bin); err == nil {
		return &containerRunner{bin: bin, root: workspaceRoot}
	}
	// Try the other
	other := "docker"
	if preferred == Docker {
		other = "podman"
	}
	if _, err := exec.LookPath(other); err == nil {
		return &containerRunner{bin: other, root: workspaceRoot}
	}
	// Fallback
	return &PathJail{root: workspaceRoot}
}

// containerRunner runs commands inside a Docker/Podman container.
type containerRunner struct {
	bin  string
	root string
}

func (c *containerRunner) Backend() Type {
	if c.bin == "podman" {
		return Podman
	}
	return Docker
}

func (c *containerRunner) Run(ctx context.Context, cmdStr string, workDir string, timeout time.Duration) (Result, error) {
	containerWorkDir := "/workspace"
	if workDir != "" && workDir != c.root {
		// Map workDir relative to root
		containerWorkDir = "/workspace/" + relPath(workDir, c.root)
	}

	args := []string{
		"run", "--rm", "-i",
		"--network", "none",
		"--memory", "512m",
		"--cpus", "1",
		"-v", c.root + ":/workspace:rw",
		"-w", containerWorkDir,
		"alpine:latest",
		"sh", "-c", cmdStr,
	}

	cmd := exec.CommandContext(ctx, c.bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}
	return result, nil
}

func relPath(path, root string) string {
	if len(path) > len(root) && path[:len(root)] == root {
		return path[len(root):]
	}
	return path
}

// PathJail is the fallback: no container, just path restrictions (existing behavior).
type PathJail struct {
	root string
}

func (p *PathJail) Backend() Type { return None }

func (p *PathJail) Run(ctx context.Context, cmdStr string, workDir string, timeout time.Duration) (Result, error) {
	// This mirrors the existing bash.go behavior — exec with path jail.
	// Return "not implemented" since the tool layer already does path jail.
	return Result{}, fmt.Errorf("path-jail sandbox: use direct bash execution (no container)")
}
