package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all tau configuration with flag > env > config > default precedence.
type Config struct {
	Workspace  string            `yaml:"workspace"`
	DataDir    string            `yaml:"data_dir"`
	Provider   string            `yaml:"provider"`
	Model      string            `yaml:"model"`
	LogLevel   string            `yaml:"log_level"`
	LogFormat  string            `yaml:"log_format"`
	HealthAddr string            `yaml:"health_addr"`
	MaxTurns   int               `yaml:"max_turns"`
	NoTools    bool              `yaml:"no_tools"`
	Providers  map[string]string `yaml:"providers"` // provider name → api key env var
	Tools      map[string]bool   `yaml:"tools"`     // tool name → enabled
	Skills     struct {
		Paths []string `yaml:"paths"`
	} `yaml:"skills"`
	Sandbox string `yaml:"sandbox"` // docker, podman, none
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Workspace: "",
		DataDir:   filepath.Join(home, ".tau"),
		Provider:  "openai",
		Model:     "gpt-5.4",
		LogLevel:  "info",
		LogFormat: "text",
		MaxTurns:  10,
		NoTools:   false,
		Sandbox:   "none",
	}
}

// LoadConfig reads a YAML config file and merges with defaults.
// Returns defaults if the file doesn't exist.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // config file not found → use defaults
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	return cfg, nil
}

// ConfigPath returns the default config path: $HOME/.tau/tau.yaml
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tau", "tau.yaml")
}
