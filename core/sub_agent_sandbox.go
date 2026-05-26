package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// SandboxSpec defines isolation parameters for sub-agent execution.
type SandboxSpec struct {
	Root            string        // filesystem root (default: /tmp/tau-sandbox-{id}/)
	AllowedCommands []string      // command whitelist (e.g., ["go", "python", "grep"])
	Network         string        // "none", "loopback", "full" (default: "none")
	Timeout         time.Duration // max execution time (default: 300s)
	MaxDiskMB       int64         // max disk usage in MB (default: 100)
}

// DefaultSandbox returns a SandboxSpec with safe defaults.
func DefaultSandbox(id string) *SandboxSpec {
	return &SandboxSpec{
		Root:            filepath.Join(os.TempDir(), "tau-sandbox-"+id),
		AllowedCommands: []string{"echo", "cat", "ls", "go", "python", "node", "grep", "find", "git", "tau"},
		Network:         "none",
		Timeout:         300 * time.Second,
		MaxDiskMB:       100,
	}
}

// Validate checks the sandbox spec for sanity.
func (s *SandboxSpec) Validate() error {
	if s.Root == "" {
		return fmt.Errorf("sandbox root required")
	}
	if s.Timeout <= 0 {
		s.Timeout = 300 * time.Second
	}
	if s.MaxDiskMB <= 0 {
		s.MaxDiskMB = 100
	}
	if s.Network == "" {
		s.Network = "none"
	}
	if len(s.AllowedCommands) == 0 {
		s.AllowedCommands = []string{"echo", "cat", "ls"}
	}
	return nil
}

// Setup creates the sandbox root directory.
func (s *SandboxSpec) Setup() error {
	if err := os.MkdirAll(s.Root, 0755); err != nil {
		return fmt.Errorf("sandbox setup: %w", err)
	}
	return nil
}

// IsCommandAllowed checks if a command is in the whitelist.
func (s *SandboxSpec) IsCommandAllowed(cmd string) bool {
	base := filepath.Base(cmd)
	for _, allowed := range s.AllowedCommands {
		if base == allowed {
			return true
		}
	}
	return false
}

// IsPathAllowed checks if a path is within the sandbox root.
func (s *SandboxSpec) IsPathAllowed(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rootAbs, _ := filepath.Abs(s.Root)
	return strings.HasPrefix(abs, rootAbs)
}

// WrapCmd wraps an exec.Cmd with sandbox restrictions.
func (s *SandboxSpec) WrapCmd(cmd *exec.Cmd) error {
	if err := s.Validate(); err != nil {
		return err
	}

	base := filepath.Base(cmd.Args[0])
	if !s.IsCommandAllowed(base) {
		return fmt.Errorf("sandbox: command %q not in whitelist", base)
	}

	if cmd.Dir == "" || !s.IsPathAllowed(cmd.Dir) {
		cmd.Dir = s.Root
	}

	if s.Network == "none" {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	return nil
}

// Cleanup removes the sandbox root.
func (s *SandboxSpec) Cleanup() error {
	if s.Root != "" && strings.HasPrefix(s.Root, os.TempDir()) {
		return os.RemoveAll(s.Root)
	}
	return nil
}
