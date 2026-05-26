package coding

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding/prompts"
	"github.com/akzj/tau/pkg/coding/tools"
	"github.com/akzj/tau/pkg/sandbox"
)

// CodingSession wraps a core.Session with coding-agent defaults.
type CodingSession struct {
	*core.Session
	WorkspaceRoot string
}

// CodingSessionOptions configures a coding session.
type CodingSessionOptions struct {
	WorkspaceRoot string
	SystemPrompt  core.SystemPromptFn
	Provider      core.Provider
	DefaultModel  core.ModelSpec
}

// NewCodingSession creates a session with all 7 coding tools registered.
func NewCodingSession(ctx context.Context, opts CodingSessionOptions) (*CodingSession, error) {
	// Set sandbox root
	tools.WorkspaceRoot = opts.WorkspaceRoot

	// Initialize sandbox runner (Docker/Podman with PathJail fallback)
	cfg := sandbox.LoadConfig()
	tools.SandboxRunner = sandbox.Detect(cfg.PreferredBackend(), opts.WorkspaceRoot)

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     opts.Provider,
		DefaultModel: opts.DefaultModel,
		SystemPrompt: opts.SystemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	cs := &CodingSession{
		Session:       sess,
		WorkspaceRoot: opts.WorkspaceRoot,
	}

	// Register all 7 tools (tools are active by default in ToolRegistry)
	cs.Tools.Register(tools.ReadTool())
	cs.Tools.Register(tools.WriteTool())
	cs.Tools.Register(tools.EditTool())
	cs.Tools.Register(tools.BashTool())
	cs.Tools.Register(tools.GlobTool())
	cs.Tools.Register(tools.GrepTool())
	cs.Tools.Register(tools.TaskTool())
	cs.Tools.SetActive([]string{"read", "write", "edit", "bash", "glob", "grep", "task"})

	// Initialize hooks
	cs.Hooks.TransformContext = core.NewChain[[]core.Message]()
	cs.Hooks.BeforeToolCall = core.NewChain[core.ToolCallEvent]()

	// TransformContext: inject dynamic context each turn
	cs.Hooks.TransformContext.Add(func(ctx context.Context, msgs []core.Message) ([]core.Message, error) {
		gitStatus := getGitStatus(opts.WorkspaceRoot)
		block, err := prompts.BuildDynamicContext(prompts.DynamicInput{
			GitStatus:    gitStatus,
			WorkingFiles: "",
		})
		if err != nil {
			return msgs, nil
		}
		ctxMsg := core.Message{
			Role:    core.RoleSystem,
			Content: block,
		}
		return append([]core.Message{ctxMsg}, msgs...), nil
	})

	// BeforeToolCall: permission gate for destructive operations
	cs.Hooks.BeforeToolCall.Add(func(ctx context.Context, ev core.ToolCallEvent) (core.ToolCallEvent, error) {
		// Read-only tools: always allowed
		switch ev.ToolName {
		case "read", "glob", "grep", "task":
			return ev, nil
		}

		// Destructive tools: bash/write/edit — log and allow for now
		// In interactive mode, this would prompt the user for confirmation.
		// The hook preserves the event but could reject by returning an error.
		fmt.Fprintf(os.Stderr, "  [gate] allowing %s\n", ev.ToolName)
		return ev, nil
	})

	// Fire session start event
	sess.EventBus.Emit(core.Event{Type: core.EvtSessionStart, Payload: sess.ID})

	return cs, nil
}

// getGitStatus returns a short git status string for dynamic context injection.
func getGitStatus(workspaceRoot string) string {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = workspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return "(not a git repo or git not available)"
	}
	status := strings.TrimSpace(string(out))
	if status == "" {
		return "(clean)"
	}
	return "```\n" + status + "\n```"
}
