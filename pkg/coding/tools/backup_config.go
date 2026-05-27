package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// BackupConfigTool backs up and restores configuration files.
//
// Parameters:
//
//	action     (string, required) — backup | restore | list
//	path       (string, required) — file or directory to back up
//	backup_dir (string, optional) — directory for backups (default: .tau/backups in path)
func BackupConfigTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: backup, restore, list"},
			"path": {"type": "string", "description": "File or directory to backup"},
			"backup_dir": {"type": "string", "description": "Directory for storing backups"}
		},
		"required": ["action", "path"]
	}`)

	return core.Tool{
		Name:        "backup_config",
		Description: "Backup and restore configuration files (.env, .yaml, .json, .toml). Creates timestamped backups, lists available backups, restores from backup.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action    string `json:"action"`
				Path      string `json:"path"`
				BackupDir string `json:"backup_dir"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.Action == "" {
				return core.ToolResult{}, fmt.Errorf("action required (backup/restore/list)")
			}

			resolved, err := ResolvePath(args.Path)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve path: %w", err)
			}

			var backupDir string
			if args.BackupDir != "" {
				backupDir, err = ResolvePath(args.BackupDir)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("resolve backup_dir: %w", err)
				}
			} else {
				backupDir = filepath.Join(filepath.Dir(resolved), ".tau", "backups")
			}

			switch args.Action {
			case "backup":
				return runBackup(resolved, backupDir)
			case "restore":
				return runRestore(resolved, backupDir)
			case "list":
				return runBackupList(resolved, backupDir)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use backup/restore/list)", args.Action)
			}
		},
	}
}

// configExtensions are the file extensions considered config files.
var configExtensions = map[string]bool{
	".env":  true,
	".yaml": true,
	".yml":  true,
	".json": true,
	".toml": true,
	".ini":  true,
	".cfg":  true,
	".conf": true,
}

func runBackup(path string, backupDir string) (core.ToolResult, error) {
	// Determine which files to back up
	var files []string

	info, err := os.Stat(path)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("stat path: %w", err)
	}

	if info.IsDir() {
		// Walk directory for config files
		filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				// Skip backup dir itself
				if strings.Contains(p, ".tau") {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(fi.Name()))
			if configExtensions[ext] {
				files = append(files, p)
			}
			return nil
		})
	} else {
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if configExtensions[ext] {
			files = append(files, path)
		} else {
			return core.ToolResult{}, fmt.Errorf("not a recognized config file: %s", path)
		}
	}

	if len(files) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No config files found to back up."}},
			Details: map[string]any{"path": path, "count": 0, "success": true},
		}, nil
	}

	// Create backup dir with timestamp
	timestamp := time.Now().Format("2006-01-02T150405")
	sessionDir := filepath.Join(backupDir, "backup_"+timestamp)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return core.ToolResult{}, fmt.Errorf("create backup dir: %w", err)
	}

	// Create manifest
	type backupEntry struct {
		Original string `json:"original"`
		Backup   string `json:"backup"`
		Size     int64  `json:"size"`
	}
	var manifest []backupEntry

	for _, f := range files {
		relPath, _ := filepath.Rel(path, f)
		if info, _ := os.Stat(path); info != nil && !info.IsDir() {
			relPath = filepath.Base(f)
		}
		backupPath := filepath.Join(sessionDir, relPath)
		if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
			continue
		}
		if err := copyFile(f, backupPath); err != nil {
			continue
		}
		fi, _ := os.Stat(f)
		var size int64
		if fi != nil {
			size = fi.Size()
		}
		manifest = append(manifest, backupEntry{
			Original: f,
			Backup:   relPath,
			Size:     size,
		})
	}

	// Write manifest
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(sessionDir, "manifest.json"), manifestData, 0644)

	output := fmt.Sprintf("## Backup Created\n\nTimestamp: %s\nFiles backed up: %d\nLocation: %s\n\n",
		timestamp, len(manifest), sessionDir)
	for _, e := range manifest {
		output += fmt.Sprintf("  %s (%d bytes)\n", e.Backup, e.Size)
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"path":       path,
			"backup_dir": sessionDir,
			"timestamp":  timestamp,
			"count":      len(manifest),
			"entries":    manifest,
			"success":    true,
		},
	}, nil
}

func runRestore(path string, backupDir string) (core.ToolResult, error) {
	// List available backups
	backups, err := listBackupSessions(backupDir)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("list backups: %w", err)
	}
	if len(backups) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No backups found in " + backupDir}},
			Details: map[string]any{"backup_dir": backupDir, "success": false},
		}, nil
	}

	// Use the latest backup
	latest := backups[len(backups)-1]
	manifestPath := filepath.Join(backupDir, latest, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("read manifest: %w", err)
	}

	var manifest []struct {
		Original string `json:"original"`
		Backup   string `json:"backup"`
		Size     int64  `json:"size"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return core.ToolResult{}, fmt.Errorf("parse manifest: %w", err)
	}

	var restored []string
	for _, e := range manifest {
		srcPath := filepath.Join(backupDir, latest, e.Backup)
		var destPath string
		if info, _ := os.Stat(path); info != nil && !info.IsDir() {
			destPath = path
		} else {
			destPath = filepath.Join(path, e.Backup)
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			continue
		}
		if err := copyFile(srcPath, destPath); err != nil {
			continue
		}
		restored = append(restored, e.Backup)
	}

	output := fmt.Sprintf("## Restore Complete\n\nSource: %s\nFiles restored: %d\n\n", latest, len(restored))
	for _, r := range restored {
		output += fmt.Sprintf("  ✓ %s\n", r)
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"path":       path,
			"backup":     latest,
			"count":      len(restored),
			"files":      restored,
			"success":    true,
		},
	}, nil
}

func runBackupList(path string, backupDir string) (core.ToolResult, error) {
	backups, err := listBackupSessions(backupDir)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("list backups: %w", err)
	}

	if len(backups) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No backups found in " + backupDir}},
			Details: map[string]any{"backup_dir": backupDir, "count": 0, "success": true},
		}, nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("## Backup List: %s\n", backupDir))
	for _, b := range backups {
		// Try to read manifest for details
		manifestPath := filepath.Join(backupDir, b, "manifest.json")
		fileCount := 0
		if data, err := os.ReadFile(manifestPath); err == nil {
			var man []any
			if json.Unmarshal(data, &man) == nil {
				fileCount = len(man)
			}
		}
		// Parse timestamp from name
		ts := strings.TrimPrefix(b, "backup_")
		lines = append(lines, fmt.Sprintf("  %s — %d files", ts, fileCount))
	}

	output := strings.Join(lines, "\n")

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"backup_dir": backupDir,
			"count":      len(backups),
			"backups":    backups,
			"success":    true,
		},
	}, nil
}

func listBackupSessions(backupDir string) ([]string, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "backup_") {
			backups = append(backups, e.Name())
		}
	}
	sort.Strings(backups)
	return backups, nil
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}
