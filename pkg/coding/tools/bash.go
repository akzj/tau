package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/akzj/tau/core"
)

// gitDestructiveOps is the set of git commands that should be blocked or warned.
var gitDestructiveOps = []struct {
	pattern string // substring to match
	block   bool   // true = reject, false = warn
	message string
}{
	{"git push --force", true, "force push to remote"},
	{"git push -f", true, "force push to remote"},
	{"git push --force-with-lease", false, "force push with lease (safer)"},
	{"git reset --hard", true, "hard reset (discards working tree)"},
	{"git clean -fd", true, "force clean untracked files"},
	{"git clean -fdx", true, "force clean including ignored"},
	{"git commit --amend", false, "amend commit"},
	{"git rebase --hard", true, "hard rebase"},
}

// CheckGitCommand checks a command string against gitDestructiveOps.
// Returns (blocked, message) — blocked=true means the command should be rejected.
func CheckGitCommand(cmd string) (blocked bool, msg string) {
	cmdLower := strings.TrimSpace(cmd)
	for _, op := range gitDestructiveOps {
		if strings.Contains(cmdLower, op.pattern) {
			return op.block, op.message
		}
	}
	return false, ""
}

// bashPrepared holds validated params for the bash three-phase flow.
type bashPrepared struct {
	Command        string
	WorkDir        string
	TimeoutSeconds int
}
// bashThreePhase implements core.ThreePhaseTool for bash.
type bashThreePhase struct{}

func (b *bashThreePhase) PrepareArgsRaw(rawArgs json.RawMessage) (json.RawMessage, error) {
	return rawArgs, nil // pass-through: bash handles JSON args directly
}

func (b *bashThreePhase) Prepare(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
	var args struct {
		Command        string `json:"command"`
		WorkDir        string `json:"work_dir"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	raw, _ := json.Marshal(params)
	if err := json.Unmarshal(raw, &args); err != nil {
		return core.PreparedTool{}, err
	}

	if strings.TrimSpace(args.Command) == "" {
		return core.PreparedTool{}, fmt.Errorf("bash: empty command")
	}
	if args.TimeoutSeconds <= 0 {
		args.TimeoutSeconds = 30
	}
	if args.TimeoutSeconds > 120 {
		args.TimeoutSeconds = 120
	}

	// Git safety: block or warn on destructive git commands — must run BEFORE WorkDir resolution
	cmdLower := strings.TrimSpace(args.Command)
	for _, op := range gitDestructiveOps {
		if strings.Contains(cmdLower, op.pattern) {
			if op.block {
				return core.PreparedTool{}, fmt.Errorf("BLOCKED: %s — use safer alternatives or override via --allow-dangerous", op.message)
			}
			fmt.Fprintf(os.Stderr, "  [git safety] WARNING: %s\n", op.message)
			break
		}
	}

	workDir := WorkspaceRoot
	if args.WorkDir != "" {
		var err error
		workDir, err = ResolvePath(args.WorkDir)
		if err != nil {
			return core.PreparedTool{}, err
		}
	}

	return core.PreparedTool{
		CallID:   callID,
		ToolName: "bash",
		Params:   args,
		State:    bashPrepared{Command: args.Command, WorkDir: workDir, TimeoutSeconds: args.TimeoutSeconds},
	}, nil
}

func (b *bashThreePhase) Execute(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
	bp := prepared.State.(bashPrepared)

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(bp.TimeoutSeconds)*time.Second)
	defer cancel()

	// Route through container sandbox if available
	backend := SandboxRunner.Backend()
	if SandboxRunner != nil && backend != "local" && backend != "none" {
		result, runErr := SandboxRunner.Run(cmdCtx, bp.Command, bp.WorkDir)
		if runErr == nil {
			output := result.Stdout
			if result.Stderr != "" {
				output += "\n[stderr]\n" + result.Stderr
			}
			if result.ExitCode != 0 {
				output += fmt.Sprintf("\n[exit: %d]", result.ExitCode)
			} else {
				output += "\n[exit: 0]"
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
			}, nil
		}
		// Fall through to direct exec if container fails
	}

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", bp.Command)
	cmd.Dir = bp.WorkDir
	cmd.Env = FilteredEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[stderr]\n" + stderr.String()
	}

	if truncated, _ := TruncateOutput(output); truncated != output {
		output = truncated
	}

	var status string
	if runErr != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			status = fmt.Sprintf("[timeout after %ds]", bp.TimeoutSeconds)
		} else if exitErr, ok := runErr.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if ws.Signaled() {
					status = fmt.Sprintf("[signal: %s]", ws.Signal())
				} else {
					status = fmt.Sprintf("[exit: %d]", ws.ExitStatus())
				}
			} else {
				status = fmt.Sprintf("[exit: error — %v]", runErr)
			}
		} else {
			status = fmt.Sprintf("[exit: error — %v]", runErr)
		}
	} else {
		status = "[exit: 0]"
	}

	statusOverhead := len(status) + 1
	if len(output)+statusOverhead > OutputCap {
		cut := OutputCap - statusOverhead - 15
		if cut < 0 {
			cut = 0
		}
		output = output[:cut] + "\n... (truncated)"
	}
	output += "\n" + status

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{"exit_code": status, "stdout_len": stdout.Len(), "stderr_len": stderr.Len()},
	}, nil
}

func (b *bashThreePhase) Finalize(ctx context.Context, prepared core.PreparedTool, result core.ToolResult) error {
	// No temp files to clean for the local exec path.
	return nil
}

// BashTool creates a bash command execution tool.
//
// Parameters:
//   command         (string, required) — the bash command to execute
//   work_dir        (string, optional, default: workspace root) — working directory for the command
//   timeout_seconds (int, optional, default 30, max 120) — command timeout in seconds
//
// Environment is restricted to a whitelist (PATH, HOME, SHELL, USER).
// Output is capped at 64KiB. Exit code, signal, and timeout are reported.
// Destructive git operations (push --force, reset --hard, clean -fd) are blocked.
func BashTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Bash command to execute"},
			"work_dir": {"type": "string", "description": "Working directory relative to workspace root (optional)"},
			"timeout_seconds": {"type": "integer", "description": "Command timeout in seconds (max 120, default 30)"}
		},
		"required": ["command"]
	}`)

	return core.Tool{
		Name:        "bash",
		Description: "Execute a bash command in the workspace.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		ThreePhase:  &bashThreePhase{},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			// Single-stage fallback delegates to three-phase internally.
			tp := &bashThreePhase{}
			prepared, err := tp.Prepare(ctx, callID, params)
			if err != nil {
				return core.ToolResult{}, err
			}
			return tp.Execute(ctx, prepared, onUpdate)
		},
	}
}
