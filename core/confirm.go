package core

import (
	"fmt"
	"os"
	"strings"
)

// ConfirmConfig controls tool confirmation behavior.
type ConfirmConfig struct {
	Yes            bool // skip all confirmations (--yes flag)
	NonInteractive bool // auto-deny in non-interactive mode
}

// IsDangerous returns true if the tool can modify files, execute commands, or make network requests.
func IsDangerous(toolName string) bool {
	dangerous := map[string]bool{
		"write": true, "edit": true, "bash": true,
		"write_file": true, "exec_command": true, "delete_file": true,
		"git": true, "git_diff": true,
		"web_fetch": true, "web_search": true, "http_request": true,
		"task_tracker": true, "spawn_agent": true,
	}
	return dangerous[toolName]
}

// Confirm prompts the user for confirmation before executing a dangerous tool.
// Returns true if the tool should proceed, false if it should be skipped.
func Confirm(cfg ConfirmConfig, toolName string, params any) bool {
	if cfg.Yes {
		return true
	}
	if cfg.NonInteractive {
		return false
	}

	fmt.Fprintf(os.Stderr, "\n⚠️  Tool: %s\n   Confirm execution? [y/N]: ", toolName)
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

// LoadConfirmConfig reads confirmation configuration from env and flags.
func LoadConfirmConfig(yesFlag bool) ConfirmConfig {
	if yesFlag {
		return ConfirmConfig{Yes: true}
	}
	if os.Getenv("TAU_YES") != "" {
		return ConfirmConfig{Yes: true}
	}
	if !isTerminal() {
		return ConfirmConfig{NonInteractive: true}
	}
	return ConfirmConfig{}
}

// isTerminal returns true if stdin is a terminal.
func isTerminal() bool {
	fi, _ := os.Stdin.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}
