package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/sandbox"
)

// bashPrepared holds validated params for the bash three-phase flow.
type bashPrepared struct {
	Command        string
	WorkDir        string
	TimeoutSeconds int
}

// bashThreePhase implements core.ThreePhaseTool for bash.
type bashThreePhase struct{}

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
	if SandboxRunner != nil && SandboxRunner.Backend() != sandbox.None {
		result, runErr := SandboxRunner.Run(cmdCtx, bp.Command, bp.WorkDir, time.Duration(bp.TimeoutSeconds)*time.Second)
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
	}, nil
}

func (b *bashThreePhase) Finalize(ctx context.Context, prepared core.PreparedTool, result core.ToolResult) error {
	// No temp files to clean for the local exec path.
	return nil
}

// BashTool creates a bash-execution tool.
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
