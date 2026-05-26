package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/pkg/sandbox"
)

// WorkspaceRoot is set at session init; all path jails are relative to this.
var WorkspaceRoot string

// SandboxRunner is an optional container runner. Set at session init.
// When nil or Backend() returns None, the bash tool falls through to direct exec.
var SandboxRunner sandbox.Runner

// ResolvePath resolves a relative path against the workspace root.
// Rejects paths that escape the workspace (.., symlinks, absolute paths).
func ResolvePath(relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths not allowed: %s", relPath)
	}
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path escapes workspace: %s", relPath)
	}
	resolved := filepath.Join(WorkspaceRoot, clean)
	// Verify resolved path is within workspace
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("cannot resolve: %w", err)
	}
	absRoot, err := filepath.Abs(WorkspaceRoot)
	if err != nil {
		return "", fmt.Errorf("cannot resolve root: %w", err)
	}
	if !strings.HasPrefix(absResolved, absRoot+string(filepath.Separator)) &&
		absResolved != absRoot {
		return "", fmt.Errorf("path escapes workspace: %s", relPath)
	}
	return resolved, nil
}

// AllowedEnv is the env allowlist for Bash.
var AllowedEnv = []string{"PATH", "HOME", "SHELL", "USER", "LANG", "PWD", "TERM"}

// FilteredEnv returns environment variables restricted to AllowedEnv.
func FilteredEnv() []string {
	var env []string
	for _, key := range AllowedEnv {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+val)
		}
	}
	return env
}

// OutputCap is the max bytes for Read/Bash output.
const OutputCap = 64 * 1024 // 64KiB
