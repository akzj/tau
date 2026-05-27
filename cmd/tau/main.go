package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
	"github.com/akzj/tau/pkg/coding/tools"
	"github.com/akzj/tau/pkg/persist"
	"github.com/akzj/tau/pkg/provider"
	"github.com/akzj/tau/pkg/tui"
	"github.com/akzj/tau/pkg/webui"
	"github.com/akzj/tau/server"
)

// ---- Flag variables ----

var (
	cfgModel         string
	cfgProvider      string
	cfgConfig        string
	cfgMaxTurns      int
	cfgYes           bool
	cfgStream        bool
	cfgAgents        int
	cfgRateLimit     int
	cfgTUI           bool
	cfgWebUI         bool
	cfgAddr          string
	cfgResume        string
	cfgSteer         string
	cfgSandbox       bool
	cfgSandboxRoot   string
	cfgSandboxNetwork string
	cfgMaxTokens     int
	cfgCtxStrategy   string
	cfgLogLevel      string
	cfgLogFormat     string
	cfgPluginDir     string
	cfgMaxSubAgents  int
	cfgWorkspace     string
	cfgVerbose       bool
	cfgNoTools       bool
	cfgNoSkills      bool
	cfgStrategy      string
	cfgHealthAddr    string
	cfgMetricsAddr   string
	cfgListSessions  bool
	cfgListModels    bool
	cfgListTools     bool
	cfgReflect       bool
	cfgReflectDepth  int
	cfgListSkills    bool
	cfgVersion       bool

	// Subcommand-local flags
	servePort   string
	serveHost   string
	serveAPIKey string
	evalJSON    bool
)

// ---- rootCmd ----

var rootCmd = &cobra.Command{
	Use:   "tau [prompt]",
	Short: "tau — an AI coding agent",
	Long: `tau is an AI coding agent that helps you write, review, and execute code.

Run without a subcommand to start an interactive coding session.
Use "tau serve" to start the HTTP API server.
Use "tau --help" for all flags and commands.`,
	Run: runDefault,
}

// ---- Subcommands ----

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP API server",
	Long:  "Start the tau HTTP API server on the configured host:port.",
	Run:   runServe,
}

var pluginCmd = &cobra.Command{
	Use:   "plugin <list|install|remove|info> [args]",
	Short: "Plugin management",
	Long:  "Manage tau plugins: list, install, remove, or inspect.",
	Run:   runPlugin,
}

var doctorCmd = &cobra.Command{
	Use:   "doctor [config-path]",
	Short: "System diagnostics",
	Long:  "Run system diagnostics to verify your tau installation.",
	Run:   runDoctor,
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion script",
	Long:  "Generate a shell completion script for bash, zsh, or fish.",
	Run:   runCompletion,
}

var evalCmd = &cobra.Command{
	Use:   "eval [suite-dir]",
	Short: "Run evaluation suite",
	Long:  "Run an evaluation suite against the current provider/model.",
	Run:   runEval,
}

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Estimate token costs",
	Long:  "Estimate token costs for a session or model (not yet implemented).",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("cost: not yet implemented")
	},
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Session management",
	Long:  "Manage tau sessions (not yet implemented).",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("session: not yet implemented")
	},
}

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Memory operations",
	Long:  "Manage tau long-term memory (not yet implemented).",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("memory: not yet implemented")
	},
}

var orchestrateCmd = &cobra.Command{
	Use:   "orchestrate <task1> <task2> ...",
	Short: "Multi-agent parallel task execution",
	Long:  "Execute multiple tasks in parallel using a pool of agents.",
	Run:   runOrchestrate,
}

// ---- init ----

func init() {
	// --- Persistent (global) flags ---
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfgModel, "model", "gpt-5.4", "Model name")
	pf.StringVar(&cfgProvider, "provider", "openai", "Provider (openai or anthropic)")
	pf.StringVar(&cfgConfig, "config", "", "Config file path (default: ~/.tau/tau.yaml)")
	pf.IntVar(&cfgMaxTurns, "max-turns", 10, "Max agent turns")
	pf.BoolVar(&cfgYes, "yes", false, "Auto-confirm all dangerous tool operations (TAU_YES env also supported)")
	pf.BoolVar(&cfgStream, "stream", false, "Enable streaming output mode")
	pf.IntVar(&cfgAgents, "agents", 1, "Number of parallel agent instances (1 = single agent)")
	pf.IntVar(&cfgRateLimit, "rate-limit", 0, "Provider rate limit (requests/sec, 0=unlimited)")
	pf.BoolVar(&cfgTUI, "tui", false, "Launch Terminal UI")
	pf.BoolVar(&cfgWebUI, "webui", false, "Launch Web UI")
	pf.StringVar(&cfgAddr, "addr", ":8080", "Web UI listen address")
	pf.StringVar(&cfgResume, "resume", "", "Resume a saved session by ID")
	pf.StringVar(&cfgSteer, "steer", "", "Inject a steer instruction (for use with --resume)")
	pf.StringVar(&cfgLogLevel, "log-level", "info", "Log level: debug, info, warn, error")
	pf.StringVar(&cfgLogFormat, "log-format", "text", "Log format: text, json")
	pf.StringVar(&cfgHealthAddr, "health-addr", "", "Health check listen address (e.g., :8081)")
	pf.StringVar(&cfgMetricsAddr, "metrics-addr", "", "Metrics listen address (e.g., :9090)")
	pf.StringVar(&cfgPluginDir, "plugin-dir", "", "Plugin directory (default: $TAU_PLUGIN_DIR or ~/.tau/plugins)")
	pf.IntVar(&cfgMaxSubAgents, "max-sub-agents", 4, "Maximum concurrent sub-agents")
	pf.BoolVar(&cfgSandbox, "sandbox", true, "Enable sandbox isolation for sub-agents")
	pf.StringVar(&cfgSandboxRoot, "sandbox-root", "", "Sandbox root directory")
	pf.StringVar(&cfgSandboxNetwork, "sandbox-network", "none", "Sandbox network: none, loopback, full")
	pf.IntVar(&cfgMaxTokens, "max-tokens", 128000, "Maximum token budget for context window")
	pf.StringVar(&cfgCtxStrategy, "ctx-strategy", "sliding", "Context strategy: sliding, truncate, summarize")
	pf.StringVar(&cfgWorkspace, "workspace", "", "Workspace root (default: cwd)")
	pf.BoolVar(&cfgVerbose, "verbose", false, "Verbose output")
	pf.BoolVar(&cfgNoTools, "no-tools", false, "Disable all tools")
	pf.BoolVar(&cfgNoSkills, "no-skills", false, "Disable all skills")
	pf.StringVar(&cfgStrategy, "strategy", "react", "Reasoning strategy: react, plan-execute, cot")
	pf.BoolVar(&cfgReflect, "reflect", false, "Enable reflection/self-correction loop")
	pf.IntVar(&cfgReflectDepth, "reflect-depth", 2, "Reflection correction rounds (default 2)")
	pf.BoolVar(&cfgListSessions, "list-sessions", false, "List saved sessions")
	pf.BoolVar(&cfgListModels, "list-models", false, "List available models")
	pf.BoolVar(&cfgListTools, "list-tools", false, "List available tools")
	pf.BoolVar(&cfgListSkills, "list-skills", false, "List available skills")
	pf.BoolVar(&cfgVersion, "version", false, "Print version and exit")

	// --- Subcommand-local flags ---
	serveCmd.Flags().StringVar(&servePort, "port", "8080", "HTTP listen port")
	serveCmd.Flags().StringVar(&serveHost, "host", "localhost", "HTTP listen host")
	serveCmd.Flags().StringVar(&serveAPIKey, "api-key", "", "API key for authentication")
	evalCmd.Flags().BoolVar(&evalJSON, "json", false, "Output results as JSON")

	// --- Register subcommands ---
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(pluginCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(costCmd)
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(orchestrateCmd)
}

// ---- Helpers ----

// loadConfig loads the config file and applies values as defaults (CLI flags override).
// Returns workspace root.
func loadConfig() (wsRoot string) {
	cfgPath := cfgConfig
	if cfgPath == "" {
		cfgPath = core.ConfigPath()
	}
	cfg, err := core.LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// Apply config values as defaults (CLI flags override)
	if cfgWorkspace == "" && cfg.Workspace != "" {
		cfgWorkspace = cfg.Workspace
	}
	if cfgModel == "gpt-5.4" && cfg.Model != "gpt-5.4" {
		cfgModel = cfg.Model
	}
	if cfgLogLevel == "info" && cfg.LogLevel != "" {
		cfgLogLevel = cfg.LogLevel
	}
	if cfgLogFormat == "text" && cfg.LogFormat != "" {
		cfgLogFormat = cfg.LogFormat
	}
	if cfgMaxTurns == 10 && cfg.MaxTurns != 0 {
		cfgMaxTurns = cfg.MaxTurns
	}
	if cfg.NoTools {
		cfgNoTools = true
	}

	// Reserved for future integration
	_ = cfgMaxSubAgents
	_ = cfgSandbox
	_ = cfgSandboxRoot
	_ = cfgSandboxNetwork
	_ = cfgMaxTokens
	_ = cfgCtxStrategy
	_ = cfgYes
	_ = cfgStrategy
	_ = cfgStream

	if cfgRateLimit > 0 {
		core.SetGlobalRateLimit(cfgRateLimit)
	}

	core.InitLogger(cfgLogLevel, cfgLogFormat)

	// Resolve workspace
	wsRoot = cfgWorkspace
	if wsRoot == "" {
		var err error
		wsRoot, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "getcwd: %v\n", err)
			os.Exit(1)
		}
	}
	return wsRoot
}

// resolveProvider loads the provider for the configured model, returning
// provider instance, model name, and WireAPI.
func resolveProvider() (core.Provider, provider.ModelInfo, core.WireAPI) {
	loader := provider.NewProviderLoader()
	modelReg, err := provider.LoadModelRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "models: %v\n", err)
		os.Exit(1)
	}

	modelInfo, ok := modelReg.Lookup(cfgModel)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown model: %s\n", cfgModel)
		fmt.Fprintf(os.Stderr, "Available models:\n")
		for _, info := range modelReg.List("") {
			fmt.Fprintf(os.Stderr, "  %-20s %s (%s)\n", info.ID, info.Name, info.Provider)
		}
		os.Exit(1)
	}

	prov, err := loader.Load(modelInfo.Provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider %s: %v\n", modelInfo.Provider, err)
		os.Exit(1)
	}

	var api core.WireAPI
	switch modelInfo.Provider {
	case "anthropic":
		api = core.WireAnthropicMessages
	case "azure":
		api = core.WireOpenAICompletions
	case "mistral":
		api = core.WireOpenAICompletions
	case "google":
		api = core.WireGoogleGenerativeAI
	default:
		api = core.WireOpenAICompletions
	}

	return prov, modelInfo, api
}

// ---- runDefault — default interactive mode ----

func runDefault(cmd *cobra.Command, args []string) {
	// --- Exit-early flags (no config needed) ---
	if cfgVersion {
		fmt.Printf("tau %s (built %s, commit %s)\n", core.Version, core.BuildTime, core.CommitSHA)
		return
	}
	if cfgListTools {
		toolNames := []string{"read", "write", "edit", "bash", "glob", "grep", "task", "task_tracker", "web_search", "web_fetch", "workspace_diag", "list_files", "search_code", "run_tests", "git_diff", "ask_user", "lint", "format", "deps", "coverage", "rag_search", "prompt_render", "verify", "browse", "git_commit", "git_log", "git_branch"}
		fmt.Println("Available tools:")
		for _, t := range toolNames {
			fmt.Printf("  %s\n", t)
		}
		return
	}
	if cfgListSkills {
		builtins := []string{
			"code-review", "debugger", "test-writer", "refactor", "architect",
			"go-refactor", "shell-scripting", "git-workflow",
			"go-code-review", "go-debugging", "go-test-writing", "go-refactoring",
			"python-code-review", "python-debugging", "python-test-writing", "python-refactoring",
			"js-code-review", "js-debugging", "js-test-writing", "js-refactoring",
			"refactoring-patterns", "debugging-strategies", "api-design",
			"database-patterns", "cicd-patterns", "testing-strategy",
			"security-review", "code-review-intensive", "performance-optimization",
			"documentation-generation",
		}
		fmt.Println("Available skills (built-in):")
		for _, s := range builtins {
			fmt.Printf("  %s\n", s)
		}
		return
	}
	if cfgListSessions {
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
	if cfgListModels {
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

	// --- Load config ---
	wsRoot := loadConfig()

	// Get prompt from args or stdin
	var prompt string
	if len(args) > 0 {
		prompt = strings.Join(args, " ")
	} else {
		data, _ := io.ReadAll(os.Stdin)
		prompt = strings.TrimSpace(string(data))
	}

	if prompt == "" && !cfgTUI && !cfgWebUI {
		fmt.Fprintf(os.Stderr, "Usage: tau [flags] <prompt>\n")
		_ = cmd.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	// Graceful shutdown
	shutdown := core.NewShutdown()
	go core.SignalHandler(shutdown)

	// Health check server
	if cfgHealthAddr != "" {
		http.HandleFunc("/health", core.HealthHandler(7, 11))
		http.HandleFunc("/ready", core.ReadyHandler())
		go func() {
			core.Logger().Info("health: listening", "addr", cfgHealthAddr)
			if err := http.ListenAndServe(cfgHealthAddr, nil); err != nil {
				core.Logger().Error("health: listen error", "err", err)
			}
		}()
	}

	// Metrics server
	if cfgMetricsAddr != "" {
		core.RegisterTauMetrics()
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/metrics", core.MetricsHandler())
			core.Logger().Info("metrics: listening", "addr", cfgMetricsAddr)
			if err := http.ListenAndServe(cfgMetricsAddr, mux); err != nil {
				core.Logger().Error("metrics: listen error", "err", err)
			}
		}()
	}

	// Plugin discovery
	if cfgPluginDir != "" {
		os.Setenv("TAU_PLUGIN_DIR", cfgPluginDir)
	}
	if err := core.Discover(); err != nil {
		core.Logger().Warn("plugin: discover error", "err", err)
	}
	core.RegisterPlugin(&tools.ExamplePlugin{})

	// Provider
	prov, modelInfo, api := resolveProvider()
	_ = modelInfo // modelInfo.Provider already consumed in resolveProvider

	// WebUI mode
	if cfgWebUI {
		srv := webui.NewServer(cfgAddr, wsRoot, cfgModel, prov)
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "webui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// System prompt
	systemPrompt := func(s *core.Session) (string, error) {
		return fmt.Sprintf("You are tau, a coding agent. Workspace: %s. Use tools to read, write, and execute code. Always verify your changes.", wsRoot), nil
	}

	// Create session
	reflectDepth := 0
	if cfgReflect {
		reflectDepth = cfgReflectDepth
	}
	sess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
		WorkspaceRoot: wsRoot,
		SystemPrompt:  systemPrompt,
		Provider:      prov,
		DefaultModel: core.ModelSpec{
			Name: cfgModel,
			API:  api,
		},
		Strategy:     cfgStrategy,
		ReflectDepth: reflectDepth,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Cancel()

	// Resume
	if cfgResume != "" {
		msgs, err := persist.Load(cfgResume)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resume: %v\n", err)
			os.Exit(1)
		}
		sess.Session.Transcript.Append(msgs...)
		if cfgVerbose {
			fmt.Fprintf(os.Stderr, "[resumed %s] (%d messages)\n", cfgResume, len(msgs))
		}
	}

	// Steer
	if cfgSteer != "" {
		sess.Session.Steer(cfgSteer)
		if cfgVerbose {
			fmt.Fprintf(os.Stderr, "[steer] %s\n", cfgSteer)
		}
	}

	// Disable tools
	if cfgNoTools {
		sess.Tools.SetActive(nil)
	}

	loop := core.NewLoop()

	// TUI mode
	if cfgTUI {
		streamUI := make(chan core.StreamEvent, 64)
		sess.Session.StreamUI = streamUI
		tm := tui.NewModel(sess, loop, prompt, streamUI)
		p := tea.NewProgram(tm, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "tui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Main loop
	currentPrompt := prompt
	for turn := 0; turn < cfgMaxTurns; turn++ {
		if cfgVerbose {
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

		timeout := time.After(300 * time.Second)
		turnDone := false
		for !turnDone {
			select {
			case ev, ok := <-run.Events():
				if !ok {
					turnDone = true
					break
				}
				printEvent(ev, cfgVerbose)
			case <-timeout:
				fmt.Fprintln(os.Stderr, "[TIMEOUT]")
				run.Cancel()
				turnDone = true
			case <-ctx.Done():
				turnDone = true
			case <-shutdown.Ctx().Done():
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
	} else if cfgVerbose {
		fmt.Fprintf(os.Stderr, "[saved %s] (%d messages)\n", sessionID, len(msgs))
	}
	persist.SaveCWD(sessionID, wsRoot)

	fmt.Println()
}

// ---- runServe ----

func runServe(cmd *cobra.Command, args []string) {
	wsRoot := loadConfig()
	prov, _, api := resolveProvider()

	srv := server.New(server.Options{
		Port:      servePort,
		Host:      serveHost,
		APIKey:    serveAPIKey,
		Workspace: wsRoot,
		Provider:  prov,
		Model:     cfgModel,
		WireAPI:   api,
	})
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

// ---- runPlugin ----

func runPlugin(cmd *cobra.Command, args []string) {
	pluginCommand(args)
}

// ---- runDoctor ----

func runDoctor(cmd *cobra.Command, args []string) {
	doctorCommand(args)
}

// ---- runCompletion ----

func runCompletion(cmd *cobra.Command, args []string) {
	shell := "bash"
	if len(args) > 0 {
		shell = args[0]
	}
	script, err := core.GenerateCompletion(core.ShellType(shell))
	if err != nil {
		fmt.Fprintf(os.Stderr, "completion: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(script)
}

// ---- runEval ----

func runEval(cmd *cobra.Command, args []string) {
	suiteDir := "skills/evals"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		suiteDir = args[0]
	}

	suite, err := core.LoadSuite(suiteDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(1)
	}
	if len(suite.Scenarios) == 0 {
		fmt.Println("No scenarios found.")
		return
	}

	fmt.Printf("Running %d evaluations...\n\n", len(suite.Scenarios))
	var results []core.EvalResult
	for _, s := range suite.Scenarios {
		r := core.RunEval(s, func(ctx context.Context, prompt string) (<-chan core.ProviderEvent, error) {
			ch := make(chan core.ProviderEvent, 2)
			go func() {
				defer close(ch)
				ch <- core.ProviderEvent{Type: core.ProvContentDelta, ContentDelta: prompt + " processed"}
			}()
			return ch, nil
		})
		results = append(results, r)
	}

	if evalJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
	} else {
		fmt.Print(core.EvalReport(results))
	}
}

// ---- runOrchestrate ----

func runOrchestrate(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: tau orchestrate <task1> <task2> ...\n")
		return
	}

	wsRoot := loadConfig()
	prov, _, api := resolveProvider()

	pool := core.NewAgentPool(cfgAgents, prov, core.ModelSpec{Name: cfgModel, API: api})
	tasks := make([]core.DelegationTask, len(args))
	for j, a := range args {
		tasks[j] = core.DelegationTask{
			ID:       fmt.Sprintf("task-%d", j+1),
			Prompt:   a,
			MaxTurns: cfgMaxTurns,
			Timeout:  120 * time.Second,
		}
	}
	fmt.Fprintf(os.Stderr, "orchestrating %d tasks with %d agents...\n", len(tasks), cfgAgents)
	results := pool.Delegate(context.Background(), tasks)
	fmt.Println("=== Orchestration Results ===")
	for _, r := range results {
		status := "✅"
		if r.Error != "" {
			status = "❌"
		}
		resp := r.Response
		if len(resp) > 60 {
			resp = resp[:57] + "..."
		}
		fmt.Printf("%s [%s] %s (calls: %d, tokens: %d, time: %s)\n",
			status, r.TaskID, resp, r.ToolCalls, r.Tokens, r.Duration)
		if r.Error != "" {
			fmt.Printf("   error: %s\n", r.Error)
		}
	}
	_ = wsRoot // may be used by future orchestrate enhancements
}

// ---- printEvent (unchanged) ----

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

// ---- main ----

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "tau: %v\n", err)
		os.Exit(1)
	}
}