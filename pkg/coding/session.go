package coding

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding/prompts"
	"github.com/akzj/tau/pkg/coding/tools"
	"github.com/akzj/tau/pkg/sandbox"
	"github.com/akzj/tau/pkg/skills"
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
	MaxTokens     int                 // token budget (default 128000)
	CtxStrategy   core.ContextStrategy // context management strategy
	Strategy      string              // strategy name (default: react)
	ReflectDepth  int                 // reflection correction rounds (0 = disabled)
}

// NewCodingSession creates a session with all 7 coding tools registered.
func NewCodingSession(ctx context.Context, opts CodingSessionOptions) (*CodingSession, error) {
	// Set sandbox root
	tools.WorkspaceRoot = opts.WorkspaceRoot

	// Initialize sandbox runner (Docker with local fallback)
	tools.SandboxRunner = sandbox.NewRunner()

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     opts.Provider,
		DefaultModel: opts.DefaultModel,
		SystemPrompt: opts.SystemPrompt,
		MaxTokens:    opts.MaxTokens,
		CtxStrategy:  opts.CtxStrategy,
		ReflectDepth: opts.ReflectDepth,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if opts.Strategy != "" {
		sess.Strategy = core.GetStrategy(opts.Strategy)
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
	cs.Tools.Register(tools.WebSearchTool())
	cs.Tools.Register(tools.WebFetchTool())
	cs.Tools.Register(tools.TaskTrackerTool())
	cs.Tools.Register(tools.WorkspaceDiagTool())
	cs.Tools.Register(tools.ListFilesTool())
	cs.Tools.Register(tools.SearchCodeTool())
	cs.Tools.Register(tools.RunTestsTool())
	cs.Tools.Register(tools.GitDiffTool())
	cs.Tools.Register(tools.AskUserTool())
	cs.Tools.Register(tools.LintTool())
	cs.Tools.Register(tools.FormatTool())
	cs.Tools.Register(tools.DepsTool())
	cs.Tools.Register(tools.CoverageTool())
	cs.Tools.Register(tools.RAGSearchTool())
	cs.Tools.Register(tools.PromptRenderTool())
	cs.Tools.Register(tools.VerifyTool())
	cs.Tools.Register(tools.BrowseTool())
	cs.Tools.Register(tools.GitCommitTool())
	cs.Tools.Register(tools.GitLogTool())
	cs.Tools.Register(tools.GitBranchTool())
	cs.Tools.Register(tools.FindReferencesTool())
	cs.Tools.Register(tools.RunTestTool())
	cs.Tools.Register(tools.RunBenchTool())
	cs.Tools.Register(tools.FormatCodeTool())
	cs.Tools.Register(tools.LintCodeTool())
	cs.Tools.Register(tools.DockerBuildTool())
	cs.Tools.Register(tools.EnvManageTool())
	cs.Tools.Register(tools.ExecSandboxTool())
	cs.Tools.Register(tools.WebScrapeTool())
	cs.Tools.Register(tools.GitHubIssueTool())
	cs.Tools.Register(tools.GitHubPRTool())
	cs.Tools.Register(tools.GitHubSearchTool())
	cs.Tools.Register(tools.GitStashTool())
	cs.Tools.Register(tools.GitBlameTool())
	cs.Tools.Register(tools.FileSearchTool())
	cs.Tools.Register(tools.FileDiffDirTool())
	cs.Tools.Register(tools.JSONQueryTool())
	cs.Tools.Register(tools.CSVQueryTool())
	cs.Tools.Register(tools.TemplateRenderTool())
	cs.Tools.Register(tools.DocGenerateTool())
	cs.Tools.Register(tools.EnvValidateTool())
	cs.Tools.Register(tools.GitTagTool())
	cs.Tools.Register(tools.GitRevertTool())
	cs.Tools.Register(tools.GitCherryPickTool())
	cs.Tools.Register(tools.GitRebaseTool())
	cs.Tools.Register(tools.WebDownloadTool())
	cs.Tools.Register(tools.WebAPICallTool())
	cs.Tools.Register(tools.WebScreenshotTool())
	cs.Tools.Register(tools.YAMLQueryTool())
	cs.Tools.Register(tools.TOMLQueryTool())
	cs.Tools.Register(tools.XMLQueryTool())
	cs.Tools.Register(tools.FileWatchTool())
	cs.Tools.Register(tools.FileArchiveTool())
	cs.Tools.Register(tools.FileChecksumTool())
	cs.Tools.Register(tools.CodeLintTool())
	cs.Tools.Register(tools.CodeFormatMultiTool())
	cs.Tools.Register(tools.DockerPSTool())
	cs.Tools.Register(tools.DockerLogsTool())
	cs.Tools.Register(tools.ProcessListTool())
	cs.Tools.Register(tools.SlackPostTool())
	cs.Tools.Register(tools.EmailSendTool())
	cs.Tools.Register(tools.JiraIssueTool())
	cs.Tools.Register(tools.ShellCompleteTool())
	cs.Tools.Register(tools.NotionAPITool())
	cs.Tools.Register(tools.LinearAPITool())
	cs.Tools.Register(tools.DiscordPostTool())
	cs.Tools.Register(tools.TelegramSendTool())
	cs.Tools.Register(tools.GoogleDriveTool())
	cs.Tools.Register(tools.GoogleSheetsTool())
	cs.Tools.Register(tools.AWSS3Tool())
	cs.Tools.Register(tools.DockerAPITool())
	cs.Tools.Register(tools.K8sPodTool())
	cs.Tools.Register(tools.GrafanaQueryTool())
	cs.Tools.Register(tools.SentryIssueTool())
	cs.Tools.Register(tools.StripeInvoiceTool())
	cs.Tools.Register(tools.ListSymbolsTool())
	cs.Tools.Register(tools.CallHierarchyTool())
	cs.Tools.SetActive([]string{"read", "write", "edit", "bash", "glob", "grep", "task", "task_tracker", "web_search", "web_fetch", "workspace_diag", "list_files", "search_code", "run_tests", "git_diff", "ask_user", "lint", "format", "deps", "coverage", "rag_search", "prompt_render", "verify", "browse", "git_commit", "git_log", "git_branch", "find_references", "run_test", "run_bench", "format_code", "lint_code", "docker_build", "env_manage", "exec_sandbox", "web_scrape", "github_issue", "github_pr", "github_search", "git_stash", "git_blame", "file_search", "file_diff_dir", "json_query", "csv_query", "template_render", "doc_generate", "env_validate", "git_tag", "git_revert", "git_cherry_pick", "git_rebase", "web_download", "web_api_call", "web_screenshot", "yaml_query", "toml_query", "xml_query", "file_watch", "file_archive", "file_checksum", "code_lint", "code_format_multi", "docker_ps", "docker_logs", "process_list", "slack_post", "email_send", "jira_issue", "shell_complete", "notion_api", "linear_api", "discord_post", "telegram_send", "google_drive", "google_sheets", "aws_s3", "docker_build_api", "k8s_pod", "grafana_query", "sentry_issue", "stripe_invoice", "list_symbols", "call_hierarchy"})

// Register domain skills (init() in pkg/skills populates GlobalSkills)
	_ = skills.GlobalSkills.List() // ensures skills package is loaded	// Initialize hooks
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

		// Git safety for bash commands
		if ev.ToolName == "bash" {
			if args, ok := ev.Args.(map[string]any); ok {
				if cmd, ok := args["command"].(string); ok {
					if blocked, msg := tools.CheckGitCommand(cmd); blocked {
						core.Warn("git safety blocked", "reason", msg)
					} else if msg != "" {
						core.Warn("git safety warning", "reason", msg)
					}
				}
			}
		}

		// Git internal file protection for write/edit
		if ev.ToolName == "write" || ev.ToolName == "edit" {
			if args, ok := ev.Args.(map[string]any); ok {
				if path, ok := args["file_path"].(string); ok {
					if strings.Contains(path, ".git/") {
						core.Warn("git safety warning", "path", path)
					}
				}
			}
		}

		// Destructive tools: bash/write/edit — log and allow for now
		core.Debug("tool gate", "tool", ev.ToolName)
		return ev, nil
	})
	// BeforeCompaction: summarise old messages when transcript gets too large
	cs.Hooks.BeforeCompaction = core.NewLastWins[core.CompactionRequest]()
	cs.Hooks.BeforeCompaction.Set(func(ctx context.Context, req core.CompactionRequest) (*core.CompactionRequest, error) {
		oldMsgs := cs.Transcript.Slice(0, req.FirstKeptEntryID)

		var parts []string
		for _, m := range oldMsgs {
			prefix := string(m.Role)
			content := m.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			parts = append(parts, prefix+": "+content)
		}
		promptText := "Summarize this conversation history in 2-3 sentences:\n\n" + strings.Join(parts, "\n")

		// Include file ops hint for richer context
		if len(req.FileOpsHint) > 0 {
			promptText += "\n\nRecent file operations:\n" + strings.Join(req.FileOpsHint, "\n")
		}

		p, err := cs.ResolveProvider()
		if err != nil {
			return &req, nil
		}

		resp, err := p.Complete(ctx, core.CompleteRequest{
			Model:    cs.DefaultModel,
			Messages: []core.Message{{Role: core.RoleUser, Content: promptText}},
		})
		if err != nil {
			return &req, nil
		}

		req.Summary = resp.Content
		return &req, nil
	})

	// BeforeAgentStart: log turn start
	cs.Hooks.BeforeAgentStart = core.NewLastWins[core.AgentStartRequest]()
	cs.Hooks.BeforeAgentStart.Observe(func(ctx context.Context, req core.AgentStartRequest) {
		core.Debug("session: turn start", "turn", req.Turn, "input", req.Input)
	})

	// AfterToolResult: log tool completion
	cs.Hooks.AfterToolResult = core.NewChain[core.ToolResultWithError]()
	cs.Hooks.AfterToolResult.Add(func(ctx context.Context, ev core.ToolResultWithError) (core.ToolResultWithError, error) {
		if ev.Err != nil {
			core.Warn("tool error", "callID", ev.CallID, "err", ev.Err)
		}
		return ev, nil
	})

	// Fire session start event
	sess.EventBus.Emit(core.Event{Type: core.EvtSessionStart, Payload: sess.ID})
	core.Logger().Info("session: new", "workspace", opts.WorkspaceRoot)

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
