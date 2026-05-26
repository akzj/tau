//go:build !no_plugins

package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// PluginInfo holds metadata about an installed plugin.
type PluginInfo struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Path    string   `json:"path"`
	Tools   []string `json:"tools"`
	Status  string   `json:"status"` // "ok" or "error: ..."
}

// ListPlugins scans the plugin directory and returns info for each plugin.
func ListPlugins(dir string) ([]PluginInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var infos []PluginInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := inspectPlugin(path)
		if err != nil {
			infos = append(infos, PluginInfo{Name: e.Name(), Path: path, Status: fmt.Sprintf("error: %v", err)})
			continue
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// InstallPlugin copies a binary to the plugin directory and verifies it.
func InstallPlugin(src, dir string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source not found: %w", err)
	}
	name := filepath.Base(src)
	dst := filepath.Join(dir, name)

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	os.MkdirAll(dir, 0755)
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		os.Remove(dst)
		return fmt.Errorf("copy: %w", err)
	}
	dstFile.Close()

	if err := os.Chmod(dst, 0755); err != nil {
		os.Remove(dst)
		return fmt.Errorf("chmod: %w", err)
	}

	_, err = inspectPlugin(dst)
	if err != nil {
		os.Remove(dst)
		return fmt.Errorf("verify: %w", err)
	}

	return nil
}

// RemovePlugin deletes a plugin from the directory.
func RemovePlugin(name, dir string) error {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin not found: %s", name)
	}
	return os.Remove(path)
}

// inspectPlugin runs the plugin binary's tools RPC and returns its info.
func inspectPlugin(path string) (PluginInfo, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return PluginInfo{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return PluginInfo{}, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return PluginInfo{}, fmt.Errorf("start: %w", err)
	}
	defer cmd.Process.Kill()

	req := map[string]any{"method": "tools", "params": nil}
	reqJSON, _ := json.Marshal(req)
	stdin.Write(append(reqJSON, '\n'))
	stdin.Close()

	var resp struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Tools   []struct {
			Name string `json:"name"`
		} `json:"tools"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(stdout).Decode(&resp); err != nil {
		return PluginInfo{}, fmt.Errorf("parse: %w", err)
	}
	if resp.Error != "" {
		return PluginInfo{}, fmt.Errorf("plugin error: %s", resp.Error)
	}

	var toolNames []string
	for _, t := range resp.Tools {
		toolNames = append(toolNames, t.Name)
	}

	return PluginInfo{
		Name:    resp.Name,
		Version: resp.Version,
		Path:    path,
		Tools:   toolNames,
		Status:  "ok",
	}, nil
}
