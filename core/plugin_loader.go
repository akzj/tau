//go:build !no_plugins

package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// ExternalPlugin wraps a subprocess that implements the plugin JSON-RPC over stdio protocol.
type ExternalPlugin struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	mu      sync.Mutex
	name    string
	version string
}

// rawSchema adapts json.RawMessage to ToolSchema.
type rawSchema struct {
	raw json.RawMessage
}

func (s rawSchema) Marshal() (json.RawMessage, error)      { return s.raw, nil }
func (s rawSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// Discover scans $TAU_PLUGIN_DIR for plugin executables and registers them.
func Discover() error {
	dir := os.Getenv("TAU_PLUGIN_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".tau", "plugins")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("plugin discover: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if !isExec(path) {
			continue
		}

		ep, err := startPlugin(path)
		if err != nil {
			Warn("plugin start failed", "name", e.Name(), "err", err)
			continue
		}
		RegisterPlugin(ep)
	}
	return nil
}

func isExec(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}

// startPlugin launches a plugin subprocess and reads its init handshake.
func startPlugin(path string) (*ExternalPlugin, error) {
	cmd := exec.Command(path)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	ep := &ExternalPlugin{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}

	tools, err := ep.rpcCall("tools", nil)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("handshake: %w", err)
	}

	var response struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(tools, &response); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("parse handshake: %w", err)
	}

	ep.name = response.Name
	ep.version = response.Version
	return ep, nil
}

// rpcCall sends a JSON-RPC request and returns the raw result.
func (ep *ExternalPlugin) rpcCall(method string, params map[string]any) (json.RawMessage, error) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	req := map[string]any{"method": method, "params": params}
	reqJSON, _ := json.Marshal(req)
	reqJSON = append(reqJSON, '\n')

	if _, err := ep.stdin.Write(reqJSON); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	if !ep.stdout.Scan() {
		return nil, fmt.Errorf("read: %w", ep.stdout.Err())
	}

	var resp struct {
		Tools  json.RawMessage `json:"tools"`
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(ep.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}
	if method == "tools" {
		// Return full response body — caller extracts name/version/tools
		return json.RawMessage(ep.stdout.Bytes()), nil
	}
	return resp.Result, nil
}

// Name implements Plugin.
func (ep *ExternalPlugin) Name() string { return ep.name }

// Version implements Plugin.
func (ep *ExternalPlugin) Version() string { return ep.version }

// Tools implements Plugin — returns tools discovered via handshake.
func (ep *ExternalPlugin) Tools() []Tool {
	result, err := ep.rpcCall("tools", nil)
	if err != nil {
		return nil
	}

	var response struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Schema      json.RawMessage `json:"schema"`
		} `json:"tools"`
	}
	_ = json.Unmarshal(result, &response)

	var tools []Tool
	for _, t := range response.Tools {
		schema := t.Schema
		tools = append(tools, Tool{
			Name:        t.Name,
			Description: t.Description,
			Schema:      rawSchema{raw: schema},
			Execute: func(ctx context.Context, callID string, params any, onUpdate func(PartialResult)) (ToolResult, error) {
				argsJSON, _ := json.Marshal(params)
				result, err := ep.rpcCall("execute", map[string]any{
					"tool": t.Name,
					"args": json.RawMessage(argsJSON),
				})
				if err != nil {
					return ToolResult{}, err
				}
				return ToolResult{Content: []Content{{Type: "text", Text: string(result)}}}, nil
			},
		})
	}
	return tools
}

// Providers implements Plugin.
func (ep *ExternalPlugin) Providers() []Provider { return nil }
