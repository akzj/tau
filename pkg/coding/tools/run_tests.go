package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/akzj/tau/core"
)

// RunTestsTool creates a test runner with auto-detection.
func RunTestsTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Package/directory path (default: workspace root)"},
			"framework": {"type": "string", "description": "Test framework: auto, go, py, js, rust. Auto detects from project files."},
			"timeout_seconds": {"type": "integer", "description": "Test timeout (default 30, max 120)"}
		}
	}`)

	return core.Tool{
		Name:        "run_tests",
		Description: "Run tests with auto-detected framework. Supports Go, Python, JS/TS, Rust.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path           string `json:"path"`
				Framework      string `json:"framework"`
				TimeoutSeconds int    `json:"timeout_seconds"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.TimeoutSeconds <= 0 {
				args.TimeoutSeconds = 30
			}
			if args.TimeoutSeconds > 120 {
				args.TimeoutSeconds = 120
			}

			workDir := WorkspaceRoot
			if args.Path != "" && args.Path != "." {
				var err error
				workDir, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			framework := args.Framework
			if framework == "" || framework == "auto" {
				framework = detectFramework(workDir)
			}
			if framework == "" {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: "No test framework detected. Try --framework flag."}}}, nil
			}

			cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(args.TimeoutSeconds)*time.Second)
			defer cancel()

			var cmd *exec.Cmd
			switch framework {
			case "go":
				cmd = exec.CommandContext(cmdCtx, "go", "test", "-v", "-count=1", "./...")
			case "py":
				cmd = exec.CommandContext(cmdCtx, "python", "-m", "pytest", "-v")
			case "js":
				cmd = exec.CommandContext(cmdCtx, "npx", "jest", "--verbose")
			case "rust":
				cmd = exec.CommandContext(cmdCtx, "cargo", "test")
			default:
				return core.ToolResult{}, fmt.Errorf("unknown framework: %s", framework)
			}
			cmd.Dir = workDir
			cmd.Env = FilteredEnv()

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n[stderr]\n" + stderr.String()
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			exitInfo := ""
			if runErr != nil {
				if cmdCtx.Err() == context.DeadlineExceeded {
					exitInfo = "[timeout]"
				} else {
					exitInfo = fmt.Sprintf("[exit: %v]", runErr)
				}
			} else {
				exitInfo = "[pass]"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output + "\n" + exitInfo}},
				Details: map[string]any{"framework": framework, "path": args.Path},
			}, nil
		},
	}
}

func detectFramework(dir string) string {
	checks := []struct{ file, framework string }{
		{"go.mod", "go"}, {"pyproject.toml", "py"}, {"setup.py", "py"},
		{"package.json", "js"}, {"Cargo.toml", "rust"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			return c.framework
		}
	}
	return ""
}
