package core

import (
	"os"
	"os/exec"
	"testing"
)

func TestSandboxDefaults(t *testing.T) {
	s := DefaultSandbox("test")
	if s.Root == "" {
		t.Error("root should not be empty")
	}
	if s.Network != "none" {
		t.Error("default network should be 'none'")
	}
	if len(s.AllowedCommands) == 0 {
		t.Error("should have default allowed commands")
	}
}

func TestSandboxCommandWhitelist(t *testing.T) {
	s := DefaultSandbox("test")
	if !s.IsCommandAllowed("echo") {
		t.Error("echo should be allowed")
	}
	if s.IsCommandAllowed("malicious_binary") {
		t.Error("malicious_binary should not be allowed")
	}
}

func TestSandboxPathCheck(t *testing.T) {
	s := DefaultSandbox("test")
	os.MkdirAll(s.Root, 0755)
	defer os.RemoveAll(s.Root)

	if !s.IsPathAllowed(s.Root) {
		t.Error("root should be allowed")
	}
	if s.IsPathAllowed("/etc/passwd") {
		t.Error("/etc/passwd should not be allowed")
	}
}

func TestSandboxWrapCmd(t *testing.T) {
	s := DefaultSandbox("test")
	s.Setup()
	defer s.Cleanup()

	cmd := exec.Command("echo", "hello")
	if err := s.WrapCmd(cmd); err != nil {
		t.Fatalf("wrap: %v", err)
	}
	if cmd.Dir != s.Root {
		t.Errorf("expected dir %s, got %s", s.Root, cmd.Dir)
	}
}

func TestSandboxWrapCmdBlocked(t *testing.T) {
	s := DefaultSandbox("test")
	s.AllowedCommands = []string{"echo"}
	cmd := exec.Command("rm", "-rf", "/")
	if err := s.WrapCmd(cmd); err == nil {
		t.Error("expected error for blocked command 'rm'")
	}
}
