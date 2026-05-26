package sandbox

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds sandbox configuration from ~/.tau/config.yaml.
type Config struct {
	Sandbox string `yaml:"sandbox"` // "docker", "podman", or "none"
}

// LoadConfig reads config from ~/.tau/config.yaml.
// Returns defaults if the file doesn't exist.
func LoadConfig() Config {
	cfg := Config{Sandbox: "none"}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".tau", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox: config parse error: %v\n", err)
		return Config{Sandbox: "none"}
	}
	return cfg
}

// PreferredBackend converts config string to Type.
func (c Config) PreferredBackend() Type {
	switch c.Sandbox {
	case "docker":
		return Docker
	case "podman":
		return Podman
	default:
		return None
	}
}
