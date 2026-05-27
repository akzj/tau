package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadMCPConfig loads MCP server configurations from standard locations.
// Searches: project .mcp.json, project mcp.json, ~/.tau/mcp.json, TAU_MCP_CONFIG env.
func LoadMCPConfig(configPath string) (*MCPConfig, error) {
	var paths []string
	if configPath != "" {
		paths = append(paths, configPath)
	} else {
		if env := os.Getenv("TAU_MCP_CONFIG"); env != "" {
			paths = append(paths, env)
		}
		cwd, _ := os.Getwd()
		paths = append(paths, filepath.Join(cwd, ".mcp.json"))
		paths = append(paths, filepath.Join(cwd, "mcp.json"))
		home, _ := os.UserHomeDir()
		if home != "" {
			paths = append(paths, filepath.Join(home, ".tau", "mcp.json"))
		}
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg MCPConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		return &cfg, nil
	}
	return nil, nil // no config found — not an error
}

// DiscoverMCPClients loads config and connects to all configured servers.
func DiscoverMCPClients(configPath string) ([]*MCPClient, error) {
	cfg, err := LoadMCPConfig(configPath)
	if err != nil {
		return nil, err
	}
	if cfg == nil || len(cfg.MCPServers) == 0 {
		return nil, nil
	}

	var clients []*MCPClient
	for name, serverCfg := range cfg.MCPServers {
		client, err := NewMCPClient(name, serverCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[mcp] error connecting to %s: %v\n", name, err)
			continue
		}
		clients = append(clients, client)
		fmt.Fprintf(os.Stderr, "[mcp] connected to %s — %d tools\n", name, len(client.Tools()))
	}
	return clients, nil
}