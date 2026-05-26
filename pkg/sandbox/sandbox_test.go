package sandbox

import (
	"testing"
)

func TestPathJailBackend(t *testing.T) {
	pj := &PathJail{root: "/tmp"}
	if pj.Backend() != None {
		t.Errorf("expected None, got %v", pj.Backend())
	}
}

func TestPathJailRun(t *testing.T) {
	pj := &PathJail{root: "/tmp"}
	_, err := pj.Run(nil, "echo hello", "/tmp", 0)
	if err == nil {
		t.Error("expected error from PathJail.Run (not implemented)")
	}
}

func TestContainerRunnerBackend(t *testing.T) {
	cr := &containerRunner{bin: "docker", root: "/tmp"}
	if cr.Backend() != Docker {
		t.Errorf("expected Docker, got %v", cr.Backend())
	}

	cr2 := &containerRunner{bin: "podman", root: "/tmp"}
	if cr2.Backend() != Podman {
		t.Errorf("expected Podman, got %v", cr2.Backend())
	}
}

func TestDetectNone(t *testing.T) {
	r := Detect(None, "/tmp")
	if r.Backend() != None {
		t.Errorf("expected None, got %v", r.Backend())
	}
}

func TestDetectFallsBack(t *testing.T) {
	r := Detect(Docker, "/tmp")
	b := r.Backend()
	if b != Docker && b != None {
		t.Errorf("expected Docker or None (fallback), got %v", b)
	}
}

func TestTypes(t *testing.T) {
	if Docker != "docker" {
		t.Error("Docker constant mismatch")
	}
	if Podman != "podman" {
		t.Error("Podman constant mismatch")
	}
	if None != "none" {
		t.Error("None constant mismatch")
	}
}

func TestRelPath(t *testing.T) {
	tests := []struct {
		path, root, want string
	}{
		{"/home/user/project/src", "/home/user/project", "/src"},
		{"/home/user/project", "/home/user/project", "/home/user/project"},
		{"/other/path", "/home/user", "/other/path"},
	}
	for _, tc := range tests {
		got := relPath(tc.path, tc.root)
		if got != tc.want {
			t.Errorf("relPath(%q, %q) = %q, want %q", tc.path, tc.root, got, tc.want)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{Sandbox: ""}
	if cfg.PreferredBackend() != None {
		t.Errorf("empty config → None, got %v", cfg.PreferredBackend())
	}
}

func TestConfigPreferredBackend(t *testing.T) {
	tests := []struct {
		value string
		want  Type
	}{
		{"docker", Docker},
		{"podman", Podman},
		{"none", None},
		{"", None},
		{"unknown", None},
	}
	for _, tc := range tests {
		cfg := Config{Sandbox: tc.value}
		got := cfg.PreferredBackend()
		if got != tc.want {
			t.Errorf("Config{%q}.PreferredBackend() = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestResultFields(t *testing.T) {
	r := Result{
		Stdout:   "hello",
		Stderr:   "",
		ExitCode: 0,
	}
	if r.Stdout != "hello" {
		t.Error("Result.Stdout field mismatch")
	}
	if r.ExitCode != 0 {
		t.Error("Result.ExitCode field mismatch")
	}
}

func TestDetectWithRoot(t *testing.T) {
	// Detect(Docker) with a real root path
	r := Detect(Docker, "/home/user/tau/workspace")
	b := r.Backend()
	if b != Docker && b != None {
		t.Errorf("expected Docker or None, got %v", b)
	}
}
