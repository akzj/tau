package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Provider != "openai" {
		t.Errorf("expected openai, got %s", cfg.Provider)
	}
	if cfg.Model != "gpt-5.4" {
		t.Errorf("expected gpt-5.4, got %s", cfg.Model)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected info, got %s", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("expected text, got %s", cfg.LogFormat)
	}
}

func TestLoadConfigNonexistent(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/tau.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Error("expected defaults for missing file")
	}
	if cfg.Provider != "openai" {
		t.Errorf("expected openai default")
	}
}

func TestLoadConfigValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tau.yaml")
	content := `provider: anthropic
model: claude-sonnet-4-6
log_level: debug
`
	os.WriteFile(path, []byte(content), 0644)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("expected anthropic, got %s", cfg.Provider)
	}
	if cfg.Model != "claude-sonnet-4-6" {
		t.Errorf("expected claude-sonnet-4-6, got %s", cfg.Model)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug")
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tau.yaml")
	os.WriteFile(path, []byte(`{{invalid yaml!!`), 0644)
	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected parse error for invalid YAML")
	}
}

func TestConfigPath(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("config path should not be empty")
	}
}
