package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// CheckResult holds the result of a single diagnostic check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warn", "error", "skipped"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Doctor runs full-stack diagnostics.
type Doctor struct {
	Results    []CheckResult
	ConfigPath string
}

// NewDoctor creates a Doctor for the given config path.
func NewDoctor(configPath string) *Doctor {
	return &Doctor{ConfigPath: configPath}
}

// RunAll executes all diagnostic checks.
func (d *Doctor) RunAll() []CheckResult {
	d.Results = nil
	d.checkConfig()
	d.checkGoVersion()
	d.checkSessionStore()
	d.checkDiskSpace()
	d.checkGit()
	d.checkPermissions()
	return d.Results
}

func (d *Doctor) addResult(name, status, msg string) {
	d.Results = append(d.Results, CheckResult{Name: name, Status: status, Message: msg})
}

func (d *Doctor) checkConfig() {
	if d.ConfigPath == "" {
		d.ConfigPath = ConfigPath()
	}
	data, err := os.ReadFile(d.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			d.addResult("config", "warn", "Config file not found (using defaults)")
			return
		}
		d.addResult("config", "error", fmt.Sprintf("Cannot read config: %v", err))
		return
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		d.addResult("config", "error", fmt.Sprintf("Invalid YAML: %v", err))
		return
	}
	d.addResult("config", "ok", fmt.Sprintf("Valid config (%d bytes)", len(data)))
}

func (d *Doctor) checkGoVersion() {
	d.addResult("go-version", "ok", runtime.Version())
}

func (d *Doctor) checkSessionStore() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".tau", "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		d.addResult("sessions", "warn", fmt.Sprintf("Session dir not found: %s", dir))
		return
	}
	var totalSize int64
	jsonlCount := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			jsonlCount++
			if info, err := e.Info(); err == nil {
				totalSize += info.Size()
			}
		}
	}
	d.addResult("sessions", "ok", fmt.Sprintf("%d session files (%d KB)", jsonlCount, totalSize/1024))
}

func (d *Doctor) checkDiskSpace() {
	f, err := os.CreateTemp("", "tau-doctor-*")
	if err != nil {
		d.addResult("disk-space", "error", fmt.Sprintf("Cannot write to temp: %v", err))
		return
	}
	f.Close()
	os.Remove(f.Name())
	d.addResult("disk-space", "ok", "Writable")
}

func (d *Doctor) checkGit() {
	if _, err := exec.LookPath("git"); err != nil {
		d.addResult("git", "warn", "Git not found in PATH")
		return
	}
	cmd := exec.Command("git", "--version")
	out, err := cmd.Output()
	if err != nil {
		d.addResult("git", "warn", "Git found but not functional")
		return
	}
	d.addResult("git", "ok", string(bytes.TrimSpace(out)))
}

func (d *Doctor) checkPermissions() {
	home, _ := os.UserHomeDir()
	tauDir := filepath.Join(home, ".tau")
	if _, err := os.Stat(tauDir); os.IsNotExist(err) {
		d.addResult("permissions", "ok", ".tau dir does not exist yet (auto-created on first use)")
		return
	}
	testFile := filepath.Join(tauDir, ".doctor-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		d.addResult("permissions", "error", fmt.Sprintf("Cannot write to .tau dir: %v", err))
		return
	}
	os.Remove(testFile)
	d.addResult("permissions", "ok", "Read/Write .tau dir OK")
}
