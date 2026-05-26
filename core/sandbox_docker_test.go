package core

import (
	"os/exec"
	"testing"
)

func TestDockerSandboxAvailable(t *testing.T) {
	ds := NewDockerSandbox()
	if ds == nil {
		t.Error("NewDockerSandbox returned nil")
	}
	t.Logf("Docker available: %v", ds.available)
}

func TestDockerSandboxRunCommandNotAvailable(t *testing.T) {
	ds := &DockerSandbox{available: false}
	_, _, _, err := ds.RunCommand(nil, "/tmp", "echo hello")
	if err == nil {
		t.Error("expected error when docker not available")
	}
}

func TestDockerSandboxWrapCommandNotAvailable(t *testing.T) {
	ds := &DockerSandbox{available: false}
	cmd := exec.Command("echo", "hello")
	originalArgs := cmd.Args
	ds.WrapCommand(cmd)
	if len(cmd.Args) != len(originalArgs) {
		t.Error("WrapCommand should not modify command when docker unavailable")
	}
}

func TestDockerSandboxDefaults(t *testing.T) {
	ds := NewDockerSandbox()
	if ds.Image != "alpine:latest" {
		t.Error("default image should be alpine:latest")
	}
	if ds.Memory != "512m" {
		t.Error("default memory should be 512m")
	}
	if ds.Network != "none" {
		t.Error("default network should be none")
	}
}
