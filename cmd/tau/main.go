package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
	"github.com/akzj/tau/pkg/tui"
	"github.com/akzj/tau/pkg/webui"
	"github.com/akzj/tau/pkg/persist"
	"github.com/akzj/tau/pkg/provider"
)

func main() {
	// Flags
	providerName := flag.String("provider", "openai", "Provider (openai or anthropic)")
	model := flag.String("model", "gpt-5.4", "Model name")
	workspace := flag.String("workspace", "", "Workspace root (default: cwd)")
	verbose := flag.Bool("verbose", false, "Verbose output")
	noTools := flag.Bool("no-tools", false, "Disable all tools")
	maxTurns := flag.Int("max-turns", 10, "Max agent turns")
	tuiMode := flag.Bool("tui", false, "Launch Terminal UI")
	webuiMode := flag.Bool("webui", false, "Launch Web UI")
	addr := flag.String("addr", ":8080", "Web UI listen address")
	resumeID := flag.String("resume", "", "Resume a saved session by ID")
	steerMsg := flag.String("steer", "", "Inject a steer instruction (for use with --resume)")
	listSessions := flag.Bool("list-sessions", false, "List saved sessions")
	listModels := flag.Bool("list-models", false, "List available models")
	listTools := flag.Bool("list-tools", false, "List available tools")
	listSkills := flag.Bool("list-skills", false, "List available skills")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	logFormat := flag.String("log-format", "text", "Log format: text, json")
	healthAddr := flag.String("health-addr", "", "Health check listen address (e.g., :8081)")
	configPath := flag.String("config", "", "Config file path (default: ~/.tau/tau.yaml)")
	flag.Parse()

	// Load config file (flag > env > config > default)
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = core.ConfigPath()
	}
	cfg, err := core.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// Apply config values as defaults (CLI flags override)
	if *workspace == "" && cfg.Workspace != "" {
		*workspace = cfg.Workspace
	}
	if *model == "gpt-5.4" && cfg.Model != "gpt-5.4" {
		*model = cfg.Model
	}
	if *logLevel == "info" && cfg.LogLevel != "" {
		*logLevel = cfg.LogLevel
	}
	if *logFormat == "text" && cfg.LogFormat != "" {
		*logFormat = cfg.LogFormat
	}
	if *maxTurns == 10 && cfg.MaxTurns != 0 {
		*maxTurns = cfg.MaxTurns
	}
	if cfg.NoTools {
		*noTools = true
	}

	core.InitLogger(*logLevel, *logFormat)

	// Resolve workspace
	wsRoot := *workspace
	if wsRoot == "" {
		var err error
		wsRoot, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "getcwd: %v\n", err)
			os.Exit(1)
		}
	}

	// Get prompt from args or stdin
	var prompt string
	if flag.NArg() > 0 {
		prompt = strings.Join(flag.Args(), " ")
	} else {
		data, _ := io.ReadAll(os.Stdin)
		prompt = strings.TrimSpace(string(data))
	}
	if prompt == "" && !*tuiMode && !*webuiMode && !*listSessions && !*listModels && !*listTools && !*listSkills {
		fmt.Fprintf(os.Stderr, "Usage: tau [flags] <prompt>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// --list-tools: print available tools and exit
	if *listTools {
		toolNames := []string{"read", "write", "edit", "bash", "glob", "grep", "task", "task_tracker", "web_search", "web_fetch", "workspace_diag"}
		fmt.Println("Available tools:")
		for _, t := range toolNames {
			fmt.Printf("  %s\n", t)
		}
		return
	}

	// --list-skills: print available skills and exit
	if *listSkills {
		builtins := []string{
			// Built-in (8)
			"code-review", "debugger", "test-writer", "refactor", "architect",
			"go-refactor", "shell-scripting", "git-workflow",
			// Go (4)
			"go-code-review", "go-debugging", "go-test-writing", "go-refactoring",
			// Python (4)
			"python-code-review", "python-debugging", "python-test-writing", "python-refactoring",
			// JS (4)
			"js-code-review", "js-debugging", "js-test-writing", "js-refactoring",
		}
		fmt.Println("Available skills (built-in):")
		for _, s := range builtins {
			fmt.Printf("  %s\n", s)
		}
		return
	}

	// --list-sessions: print saved sessions and exit
	if *listSessions {
		sessions, err := persist.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "list: %v\n", err)
			os.Exit(1)
		}
		if len(sessions) == 0 {
			fmt.Println("No saved sessions.")
			return
		}
		fmt.Printf("%-20s %-12s %-30s %s\n", "ID", "MSGS", "CWD", "FIRST MESSAGE")
		for _, s := range sessions {
			fmt.Printf("%-20s %-12d %-30s %s\n", s.ID, s.MsgCount, s.CWD, s.FirstMsg)
		}
		return
	}

	// --list-models: print model catalog and exit
	if *listModels {
		modelReg, err := provider.LoadModelRegistry()
		if err != nil {
			fmt.Fprintf(os.Stderr, "models: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%-22s %-12s %-8s %-12s %-14s\n", "MODEL", "PROVIDER", "CTX WIN", "COST IN", "COST OUT")
		for _, info := range modelReg.List("") {
			fmt.Printf("%-22s %-12s %-8d $%-11.2f $%-13.2f\n",
				info.ID, info.Provider, info.ContextWindow, info.Cost.Input, info.Cost.Output)
		}
		return
	}

	ctx := context.Background()

	// Set up graceful shutdown (SIGINT/SIGTERM)
	shutdown := core.NewShutdown()
	go core.SignalHandler(shutdown)

	// Start health check server if requested
	if *healthAddr != "" {
		http.HandleFunc("/health", core.HealthHandler(7, 11))
		http.HandleFunc("/ready", core.ReadyHandler())
		go func() {
			core.Logger().Info("health: listening", "addr", *healthAddr)
			if err := http.ListenAndServe(*healthAddr, nil); err != nil {
				core.Logger().Error("health: listen error", "err", err)
			}
		}()
	}

	// Initialize provider system (--provider flag preserved for backward compat;
	// provider is now auto-resolved from --model via the model registry)
	_ = providerName
	loader := provider.NewProviderLoader()
	modelReg, err := provider.LoadModelRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "models: %v\n", err)
		os.Exit(1)
	}

	// Lookup model info
	modelInfo, ok := modelReg.Lookup(*model)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown model: %s\n", *model)
		fmt.Fprintf(os.Stderr, "Available models:\n")
		for _, info := range modelReg.List("") {
			fmt.Fprintf(os.Stderr, "  %-20s %s (%s)\n", info.ID, info.Name, info.Provider)
		}
		os.Exit(1)
	}

	// Load provider
	prov, err := loader.Load(modelInfo.Provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider %s: %v\n", modelInfo.Provider, err)
		os.Exit(1)
	}

	// Determine WireAPI from provider
	var api core.WireAPI
	switch modelInfo.Provider {
	case "anthropic":
		api = core.WireAnthropicMessages
	case "azure":
		api = core.WireOpenAICompletions // Azure uses same wire protocol
	case "mistral":
		api = core.WireOpenAICompletions // Mistral uses same wire protocol
	case "google":
		api = core.WireGoogleGenerativeAI
	default:
		api = core.WireOpenAICompletions
	}

	// System prompt
	systemPrompt := func(s *core.Session) (string, error) {
		return fmt.Sprintf("You are tau, a coding agent. Workspace: %s. Use tools to read, write, and execute code. Always verify your changes.", wsRoot), nil
	}

	// Create session
	sess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
		WorkspaceRoot: wsRoot,
		SystemPrompt:  systemPrompt,
		Provider:      prov,
		DefaultModel: core.ModelSpec{
			Name: *model,
			API:  api,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Cancel()

	// Resume: load saved transcript history
	if *resumeID != "" {
		msgs, err := persist.Load(*resumeID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resume: %v\n", err)
			os.Exit(1)
		}
		sess.Session.Transcript.Append(msgs...)
		if *verbose {
			fmt.Fprintf(os.Stderr, "[resumed %s] (%d messages)\n", *resumeID, len(msgs))
		}
	}

	// Inject steer instruction if provided with --resume
	if *steerMsg != "" {
		sess.Session.Steer(*steerMsg)
		if *verbose {
			fmt.Fprintf(os.Stderr, "[steer] %s\n", *steerMsg)
		}
	}

	// Disable tools if requested
	if *noTools {
		sess.Tools.SetActive(nil)
	}

	loop := core.NewLoop()

	// WebUI mode: launch HTTP+WebSocket server, skip batch loop.
	if *webuiMode {
		srv := webui.NewServer(*addr, wsRoot, *model, prov)
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "webui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// TUI mode: launch Bubble Tea, skip batch loop.
	if *tuiMode {
		tm := tui.NewModel(sess, loop, prompt)
		p := tea.NewProgram(tm, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "tui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Main loop
	currentPrompt := prompt

	for turn := 0; turn < *maxTurns; turn++ {
		if *verbose {
			fmt.Fprintf(os.Stderr, "[turn %d]\n", turn+1)
		}

		var run *core.Run
		if turn == 0 {
			run, err = loop.Prompt(ctx, sess.Session, core.UserInput{Text: currentPrompt})
		} else {
			run, err = loop.Continue(ctx, sess.Session)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Consume events
		timeout := time.After(300 * time.Second)
		turnDone := false
		for !turnDone {
			select {
			case ev, ok := <-run.Events():
				if !ok {
					turnDone = true
					break
				}
				printEvent(ev, *verbose)
			case <-timeout:
				fmt.Fprintln(os.Stderr, "[TIMEOUT]")
				run.Cancel()
				turnDone = true
			case <-ctx.Done():
				turnDone = true
			case <-shutdown.Ctx().Done():
				// Graceful shutdown — save and exit
				sessionID := persist.NewID()
				if err := persist.Save(sessionID, sess.Transcript.Messages()); err == nil {
					fmt.Fprintf(os.Stderr, "tau: session saved as %s\n", sessionID)
				}
				shutdown.MarkSaved()
				return
			}
		}
		<-run.Done()
	}

	// Save session on exit
	sessionID := persist.NewID()
	msgs := sess.Transcript.Messages()
	if err := persist.Save(sessionID, msgs); err != nil {
		fmt.Fprintf(os.Stderr, "save: %v\n", err)
	} else if *verbose {
		fmt.Fprintf(os.Stderr, "[saved %s] (%d messages)\n", sessionID, len(msgs))
	}
	persist.SaveCWD(sessionID, wsRoot) // non-fatal

	fmt.Println()
}

func printEvent(ev core.AgentEvent, verbose bool) {
	switch e := ev.(type) {
	case core.TurnStart:
		if verbose {
			fmt.Fprintf(os.Stderr, "[turn start]\n")
		}
	case core.MessageStart:
		// no print
	case core.MessageDelta:
		fmt.Print(e.ContentDelta)
	case core.MessageEnd:
		fmt.Println()
	case core.ToolCallStart:
		if verbose {
			fmt.Fprintf(os.Stderr, "[tool: %s]\n", e.ToolName)
		}
	case core.ToolCallEnd:
		if verbose {
			for _, c := range e.Result.Content {
				if c.Type == "text" && len(c.Text) > 200 {
					fmt.Fprintf(os.Stderr, "[tool result] %s...\n", c.Text[:200])
				} else if c.Type == "text" {
					fmt.Fprintf(os.Stderr, "[tool result] %s\n", c.Text)
				}
			}
		}
	case core.TurnEnd:
		if verbose {
			fmt.Fprintf(os.Stderr, "[turn end] %s\n", e.Reason)
		}
	case core.ErrorEvent:
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", e.Err)
	}
}
