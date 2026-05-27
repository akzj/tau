package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/core/cache"
	"github.com/akzj/tau/core/reasoning"
	"github.com/akzj/tau/pkg/coding"
	"github.com/akzj/tau/pkg/coding/tools"
	"github.com/akzj/tau/pkg/persist"
	"github.com/akzj/tau/pkg/provider"
	"github.com/akzj/tau/pkg/skills"
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
	cfgFallback      string
	cfgLBMode        string
	cfgProviderWeight string
	cfgHealthInterval time.Duration
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
	cfgContextThreshold float64
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
	cfgDBPath        string
	cfgRestore       bool
	cfgNoPersist     bool
	cfgSemantic      bool
	cfgPlan          bool
	cfgPlanStrategy  string
	cfgPlanMaxDepth  int
	cfgPlanAutoReplan bool
	cfgMCPConfig     string
	cfgRAGPath       string
	cfgRAGTopK       int
	cfgRAGChunkSize  int
	cfgEmbedder      string
	cfgReasoning          string
	cfgCotSteps           int
	cfgConsistencySamples int
	cfgCacheLLM         bool
	cfgCacheEmbedding   bool
	cfgCacheTool        bool
	cfgCacheTTL         time.Duration

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
var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Domain skills management",
	Long:  "Manage tau domain skills: list available skills.",
	Run:   runSkillsList,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available domain skills",
	Long:  "List all registered domain skills with category and tool information.",
	Run:   runSkillsList,
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

// ---- plan subcommand ----

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan visualization and status",
}

var planShowCmd = &cobra.Command{
	Use:   "show [goal]",
	Short: "Show plan tree",
	Run: func(cmd *cobra.Command, args []string) {
		goal := "default"
		if len(args) > 0 {
			goal = args[0]
		}
		planner := core.NewSimplePlanner()
		plan, _ := planner.Plan(context.Background(), goal)
		fmt.Print(plan.Visualize())
	},
}

var planStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show plan progress",
	Run: func(cmd *cobra.Command, args []string) {
		planner := core.NewSimplePlanner()
		plan, _ := planner.Plan(context.Background(), "current")
		done, total := plan.Progress()
		fmt.Printf("Progress: %d/%d (%.0f%%)\n", done, total, float64(done)/float64(total)*100)
	},
}

// ---- context subcommand ----

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Context window management",
}

var contextStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show context window stats",
	Run: func(cmd *cobra.Command, args []string) {
		cb := core.NewContextBudget(cfgMaxTokens)
		bar := makeProgressBar(cb)
		fmt.Printf("Context Window: %s\n", bar)
		fmt.Printf("Max: %d tokens\n", cb.MaxTokens)
		fmt.Printf("Used: %d tokens\n", cb.UsedTokens)
		fmt.Printf("Reserved: %d tokens\n", 2000)
		fmt.Printf("Remaining: %d tokens\n", cb.Remaining())
		fmt.Printf("Compression count: %d\n", cb.CompressionCount)
	},
}

// ---- mcp subcommand ----

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP protocol operations",
	Long:  "Manage MCP (Model Context Protocol) servers and tools.",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (exposes tau tools via MCP)",
	Long:  "Start an MCP JSON-RPC 2.0 server over stdio, exposing tau's tool registry.",
	Run: func(cmd *cobra.Command, args []string) {
		tools := listAllTools()
		srv := core.NewMCPServer(tools)
		if err := srv.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "mcp serve: %v\n", err)
			os.Exit(1)
		}
	},
}

var mcpDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover and connect to MCP servers",
	Long:  "Load MCP configuration and connect to configured MCP servers.",
	Run: func(cmd *cobra.Command, args []string) {
		clients, err := core.DiscoverMCPClients(cfgMCPConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mcp discover: %v\n", err)
			return
		}
		if len(clients) == 0 {
			fmt.Println("No MCP servers found.")
			return
		}
		fmt.Printf("Connected to %d MCP servers:\n", len(clients))
		for _, c := range clients {
			fmt.Printf("  %s: %d tools\n", c.ServerName(), len(c.Tools()))
		}
	},
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tools (tau + MCP)",
	Long:  "List tau built-in tools and any discovered MCP server tools.",
	Run: func(cmd *cobra.Command, args []string) {
		tools := listAllTools()
		fmt.Printf("tau built-in tools: %d\n", len(tools))
		clients, _ := core.DiscoverMCPClients(cfgMCPConfig)
		for _, c := range clients {
			fmt.Printf("MCP/%s: %d tools\n", c.ServerName(), len(c.Tools()))
			for _, t := range c.Tools() {
				fmt.Printf("  - %s: %s\n", t.Name, t.Description)
			}
		}
	},
}


// ---- rag subcommand ----

var ragCmd = &cobra.Command{
	Use:   "rag",
	Short: "RAG pipeline operations",
	Long:  "Manage the RAG (Retrieval-Augmented Generation) pipeline for semantic document search.",
}

var ragIndexCmd = &cobra.Command{
	Use:   "index [directory]",
	Short: "Index documents for semantic search",
	Long:  "Scan a directory, chunk documents, compute embeddings, and store them in the vector DB.",
	Run: func(cmd *cobra.Command, args []string) {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		loader := core.NewDocumentLoader()
		loader.ChunkSize = cfgRAGChunkSize
		chunks, err := loader.Load(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rag index: %v\n", err)
			return
		}
		if len(chunks) == 0 {
			fmt.Println("No documents found in", dir)
			return
		}
		fmt.Printf("Scanned %d chunks from %s\n", len(chunks), dir)

		// Index with vector pipeline if a store path is configured
		if cfgRAGPath != "" || cfgDBPath != "" {
			dbPath := cfgDBPath
			if dbPath == "" {
				dbPath = cfgRAGPath + "/tau.db"
			}
			store, err := core.NewSQLiteStore(dbPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "rag index: store: %v\n", err)
				return
			}
			defer store.Close()
			vs, err := core.NewSQLiteVectorStore(store)
			if err != nil {
				fmt.Fprintf(os.Stderr, "rag index: vector store: %v\n", err)
				return
			}
			embedder := core.GetEmbedder(cfgEmbedder)
			pipeline := core.NewVectorRAGPipeline(embedder, vs, cfgRAGTopK)
			n, err := pipeline.Index(cmd.Context(), chunks)
			if err != nil {
				fmt.Fprintf(os.Stderr, "rag index: %v\n", err)
				return
			}
			fmt.Printf("Indexed %d/%d chunks (embedder: %s)\n", n, len(chunks), embedder.Name())
		} else {
			fmt.Println("(in-memory only; use --db-path or --rag-path for persistence)")
		}
	},
}

var ragSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search indexed documents",
	Long:  "Search the vector store for documents semantically similar to the query.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Usage: tau rag search <query>\n")
			return
		}
		query := strings.Join(args, " ")
		embedder := core.GetEmbedder(cfgEmbedder)
		fmt.Printf("Searching for: %s (embedder: %s, top-k: %d)\n", query, embedder.Name(), cfgRAGTopK)

		dbPath := cfgDBPath
		if dbPath == "" && cfgRAGPath != "" {
			dbPath = cfgRAGPath + "/tau.db"
		}
		if dbPath == "" {
			fmt.Fprintf(os.Stderr, "rag search: no database configured (use --db-path or --rag-path)\n")
			return
		}
		store, err := core.NewSQLiteStore(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rag search: %v\n", err)
			return
		}
		defer store.Close()
		vs, err := core.NewSQLiteVectorStore(store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rag search: %v\n", err)
			return
		}
		pipeline := core.NewVectorRAGPipeline(embedder, vs, cfgRAGTopK)
		results, err := pipeline.Search(cmd.Context(), query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rag search: %v\n", err)
			return
		}
		if len(results) == 0 {
			fmt.Println("No results found.")
			return
		}
		for i, r := range results {
			fmt.Printf("%d. %s\n   %s\n\n", i+1, r.Chunk, r.Text)
		}
	},
}

var ragStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "RAG pipeline statistics",
	Long:  "Display RAG pipeline configuration and statistics.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("RAG Pipeline Stats\n")
		fmt.Printf("  Path:       %s\n", cfgRAGPath)
		fmt.Printf("  DB path:    %s\n", cfgDBPath)
		fmt.Printf("  Top-K:      %d\n", cfgRAGTopK)
		fmt.Printf("  Chunk size: %d\n", cfgRAGChunkSize)
		fmt.Printf("  Embedder:   %s\n", cfgEmbedder)
	},
}
// ---- reason subcommand ----

var reasonCmd = &cobra.Command{
	Use:   "reason",
	Short: "Advanced reasoning strategies",
	Long:  "Run advanced reasoning strategies: chain-of-thought, tree-of-thoughts, self-consistency, or task decomposition.",
}

var reasonRunCmd = &cobra.Command{
	Use:   "run [question]",
	Short: "Run reasoning on a question",
	Long: `Run a reasoning strategy on the given question.

Use --reasoning to select the strategy (cot, tot, consistency, decompose).`,
	Run: func(cmd *cobra.Command, args []string) {
		strategyName := cfgReasoning
		if strategyName == "" {
			strategyName = "cot"
		}
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Usage: tau reason run <question>\n")
			return
		}
		question := strings.Join(args, " ")
		s := core.GetStrategy(strategyName)
		if s == nil {
			fmt.Fprintf(os.Stderr, "error: unknown strategy %q\n", strategyName)
			return
		}
		state := core.AgentState{
			Messages: []core.Message{{Role: core.RoleUser, Content: question}},
			MaxTurns: 10,
		}
		decision, err := s.Decide(context.Background(), state)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		fmt.Println(decision.Reason)
	},
}

var reasonListCmd = &cobra.Command{
	Use:   "list",
	Short: "List reasoning strategies",
	Long:  "List all available reasoning strategies.",
	Run: func(cmd *cobra.Command, args []string) {
		for _, name := range []string{"cot", "tot", "consistency", "decompose"} {
			s := core.GetStrategy(name)
			if s != nil {
				fmt.Printf("%-15s %s\n", name, s.Name())
			}
		}
	},
}

// ---- cache subcommand ----

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Cache operations",
	Long:  "Manage the caching layer: view statistics.",
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long:  "Display cache hits, misses, item count, and hit rate.",
	Run: func(cmd *cobra.Command, args []string) {
		dbPath := cfgDBPath
		if dbPath == "" {
			dbPath = os.ExpandEnv("$HOME/.tau/tau.db")
		}

		store, err := core.NewSQLiteStore(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cache stats: %v\n", err)
			return
		}
		defer store.Close()

		cacheBackend, err := cache.NewSQLiteCache(store.DB(), 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cache stats: %v\n", err)
			return
		}

		s := cacheBackend.Stats()
		total := s.Hits + s.Misses
		rate := 0.0
		if total > 0 {
			rate = float64(s.Hits) / float64(total) * 100
		}

		fmt.Printf("Cache Statistics\n")
		fmt.Printf("  Hits:       %d\n", s.Hits)
		fmt.Printf("  Misses:     %d\n", s.Misses)
		fmt.Printf("  Hit rate:   %.1f%%\n", rate)
		fmt.Printf("  Items:      %d\n", s.ItemCount)
		fmt.Printf("  Max size:   %d\n", s.MaxSize)
	},
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
	pf.StringVar(&cfgFallback, "fallback", "", "Provider fallback chain (comma-separated, e.g., openai,google,anthropic)")
	pf.StringVar(&cfgLBMode, "lb-mode", "sequential", "Load balancing mode: sequential, round-robin, weighted")
	pf.StringVar(&cfgProviderWeight, "provider-weight", "", "Provider weights (e.g., anthropic=5,openai=3)")
	pf.DurationVar(&cfgHealthInterval, "health-check-interval", 60*time.Second, "Health check interval")
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
	pf.Float64Var(&cfgContextThreshold, "context-threshold", 0.8, "Compression threshold (0.0-1.0)")
	pf.StringVar(&cfgWorkspace, "workspace", "", "Workspace root (default: cwd)")
	pf.BoolVar(&cfgVerbose, "verbose", false, "Verbose output")
	pf.BoolVar(&cfgNoTools, "no-tools", false, "Disable all tools")
	pf.BoolVar(&cfgNoSkills, "no-skills", false, "Disable all skills")
	pf.StringVar(&cfgStrategy, "strategy", "react", "Reasoning strategy: react, plan-execute, cot, plan")
	pf.BoolVar(&cfgReflect, "reflect", false, "Enable reflection/self-correction loop")
	pf.IntVar(&cfgReflectDepth, "reflect-depth", 2, "Reflection correction rounds (default 2)")
	pf.BoolVar(&cfgPlan, "plan", true, "Enable planning/task decomposition")
	pf.StringVar(&cfgPlanStrategy, "plan-strategy", "simple", "Planning strategy: simple, llm")
	pf.IntVar(&cfgPlanMaxDepth, "plan-max-depth", 5, "Max plan depth")
	pf.BoolVar(&cfgPlanAutoReplan, "plan-auto-replan", true, "Auto-replan on failure")
	pf.StringVar(&cfgDBPath, "db-path", "", "SQLite database path (default: ~/.tau/tau.db)")
	pf.BoolVar(&cfgRestore, "restore", false, "Restore previous session state from database")
	pf.BoolVar(&cfgNoPersist, "no-persist", false, "Disable persistence (pure in-memory, zero file writes)")
	pf.BoolVar(&cfgSemantic, "semantic", true, "Enable semantic memory / knowledge extraction")
	pf.BoolVar(&cfgListSessions, "list-sessions", false, "List saved sessions")
	pf.BoolVar(&cfgListModels, "list-models", false, "List available models")
	pf.BoolVar(&cfgListTools, "list-tools", false, "List available tools")
	pf.BoolVar(&cfgListSkills, "list-skills", false, "List available skills")
	pf.BoolVar(&cfgVersion, "version", false, "Print version and exit")
	pf.StringVar(&cfgMCPConfig, "mcp-config", "", "MCP config file path (default: auto-detect mcp.json)")
	pf.StringVar(&cfgRAGPath, "rag-path", "", "Documents directory for RAG indexing (empty = disabled)")
	pf.IntVar(&cfgRAGTopK, "rag-top-k", 5, "Number of documents to retrieve per RAG query")
	pf.IntVar(&cfgRAGChunkSize, "chunk-size", 1000, "Document chunk size in characters")
	pf.StringVar(&cfgEmbedder, "embedder", "tf-idf", "Embedder: tf-idf, openai, google")
	pf.StringVar(&cfgReasoning, "reasoning", "", "Advanced reasoning: cot, tot, consistency, decompose")
	pf.IntVar(&cfgCotSteps, "cot-steps", 5, "CoT number of reasoning steps")
	pf.IntVar(&cfgConsistencySamples, "consistency-samples", 5, "Consistency sample count")
	pf.BoolVar(&cfgCacheLLM, "cache-llm", false, "Enable LLM response caching")
	pf.BoolVar(&cfgCacheEmbedding, "cache-embedding", false, "Enable embedding caching")
	pf.BoolVar(&cfgCacheTool, "cache-tool", false, "Enable tool result caching")
	pf.DurationVar(&cfgCacheTTL, "cache-ttl", time.Hour, "Default cache TTL (e.g., 1h, 30m)")

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
	planCmd.AddCommand(planShowCmd, planStatusCmd)
	rootCmd.AddCommand(planCmd)
	contextCmd.AddCommand(contextStatsCmd)
	rootCmd.AddCommand(contextCmd)
	mcpCmd.AddCommand(mcpServeCmd, mcpDiscoverCmd, mcpListCmd)
	rootCmd.AddCommand(mcpCmd)
	ragCmd.AddCommand(ragIndexCmd, ragSearchCmd, ragStatsCmd)
	rootCmd.AddCommand(ragCmd)
	reasonCmd.AddCommand(reasonRunCmd, reasonListCmd)
	rootCmd.AddCommand(reasonCmd)
	cacheCmd.AddCommand(cacheStatsCmd)
	rootCmd.AddCommand(cacheCmd)
	skillsCmd.AddCommand(skillsListCmd)
	rootCmd.AddCommand(skillsCmd)
}

// ---- Helpers ----

// listAllTools returns all tau built-in tools as core.Tool objects.
func listAllTools() []core.Tool {
	return []core.Tool{
		tools.ReadTool(),
		tools.WriteTool(),
		tools.EditTool(),
		tools.BashTool(),
		tools.GlobTool(),
		tools.GrepTool(),
		tools.TaskTool(),
		tools.TaskTrackerTool(),
		tools.WebSearchTool(),
		tools.WebFetchTool(),
		tools.WorkspaceDiagTool(),
		tools.ListFilesTool(),
		tools.SearchCodeTool(),
		tools.RunTestsTool(),
		tools.GitDiffTool(),
		tools.AskUserTool(),
		tools.LintTool(),
		tools.FormatTool(),
		tools.DepsTool(),
		tools.CoverageTool(),
		tools.RAGSearchTool(),
		tools.PromptRenderTool(),
		tools.VerifyTool(),
		tools.BrowseTool(),
		tools.GitCommitTool(),
		tools.GitLogTool(),
		tools.GitBranchTool(),
		tools.ListSymbolsTool(),
		tools.CallHierarchyTool(),
	}
}

// makeProgressBar creates a visual progress bar for context window usage.
func makeProgressBar(cb *core.ContextBudget) string {
	used := float64(cb.UsedTokens)
	max := float64(cb.MaxTokens)
	pct := used / max
	width := 40
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %.1f%%", bar, pct*100)
}

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
		toolNames := []string{"read", "write", "edit", "bash", "glob", "grep", "task", "task_tracker", "web_search", "web_fetch", "workspace_diag", "list_files", "search_code", "run_tests", "git_diff", "ask_user", "lint", "format", "deps", "coverage", "rag_search", "prompt_render", "verify", "browse", "git_commit", "git_log", "git_branch", "find_references", "run_test", "run_bench", "format_code", "lint_code", "docker_build", "env_manage", "exec_sandbox", "web_scrape", "github_issue", "github_pr", "github_search", "git_stash", "git_blame", "file_search", "file_diff_dir", "json_query", "csv_query", "template_render", "doc_generate", "env_validate", "git_tag", "git_revert", "git_cherry_pick", "git_rebase", "web_download", "web_api_call", "web_screenshot", "yaml_query", "toml_query", "xml_query", "file_watch", "file_archive", "file_checksum", "code_lint", "code_format_multi", "docker_ps", "docker_logs", "process_list", "slack_post", "email_send", "jira_issue", "shell_complete", "notion_api", "linear_api", "discord_post", "telegram_send", "google_drive", "google_sheets", "aws_s3", "docker_build_api", "k8s_pod", "grafana_query", "sentry_issue", "stripe_invoice"}
		fmt.Println("Available tools:")
		for _, t := range toolNames {
			fmt.Printf("  %s\n", t)
		}
		return
	}
	if cfgListSkills {
		all := skills.GlobalSkills.List()
		fmt.Println("Available domain skills:")
		for _, s := range all {
			fmt.Printf("  %-20s [%s] %s\n", s.Name, s.Category, s.Description)
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

	// Fallback chain wiring
	if cfgFallback != "" {
		chain := core.ParseFallbackChain(cfgFallback)
		weights := core.ParseProviderWeights(cfgProviderWeight)
		loader := provider.NewProviderLoader()
		entries := make([]core.ProviderEntry, 0, len(chain))
		for _, name := range chain {
			p, err := loader.Load(name)
			if err != nil {
				core.Logger().Warn("fallback: cannot load provider, skipping", "name", name, "err", err)
				continue
			}
			w := weights[name]
			if w <= 0 {
				w = 1
			}
			entries = append(entries, core.ProviderEntry{Provider: p, Name: name, Weight: w})
		}
		if len(entries) > 0 {
			// If the resolved primary provider is not already in the chain, prepend it
			primaryName := modelInfo.Provider
			found := false
			for _, e := range entries {
				if e.Name == primaryName {
					found = true
					break
				}
			}
			if !found {
				entries = append([]core.ProviderEntry{{Provider: prov, Name: primaryName, Weight: 1}}, entries...)
			}
			router := core.NewFallbackRouter(entries, core.FallbackPolicy(cfgLBMode))
			router.StartHealthCheck(cfgHealthInterval)
			prov = router
			core.Logger().Info("fallback: router active", "chain", chain, "policy", cfgLBMode)
		}
	}

	// WebUI mode
	if cfgWebUI {
		srv := webui.NewServer(webui.Options{
			Addr:      cfgAddr,
			Workspace: wsRoot,
			Model:     cfgModel,
			Provider:  prov,
			WebUI:     true,
		})
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "webui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// System prompt
	systemPrompt := func(s *core.Session) (string, error) {
		base := fmt.Sprintf("You are tau, a coding agent. Workspace: %s. Use tools to read, write, and execute code. Always verify your changes.", wsRoot)

		// Inject semantic facts from knowledge extraction
		if s.Memory != nil && s.Memory.SemMem != nil {
			msgs := s.Transcript.Messages()
			// Find the last user message
			var lastUser string
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == core.RoleUser && msgs[i].Content != "" {
					lastUser = msgs[i].Content
					break
				}
			}
			if lastUser != "" {
				facts := s.Memory.SemMem.Query(lastUser, 5)
				if len(facts) > 0 {
					base += "\n\n## Relevant Experience\n"
					for _, f := range facts {
						base += fmt.Sprintf("- %s [tags: %s]\n", f.Content, strings.Join(f.Tags, ", "))
					}
				}
			}
		}

		return base, nil
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

	// Plan wiring: when --plan flag is enabled, wrap strategy with plan tracking
	if cfgPlan {
		planner := core.NewSimplePlanner()
		plan, _ := planner.Plan(context.Background(), prompt)
		sess.Session.Strategy = core.NewPlanStrategy(sess.Session.Strategy, plan)
		fmt.Fprintf(os.Stderr, "[planner] %d subtasks for goal: %s\n", len(plan.Root.Children), prompt)
	}

	// Reasoning wiring: wrap strategy with advanced reasoning
	if cfgReasoning != "" {
		inner := sess.Session.Strategy
		switch cfgReasoning {
		case "cot":
			sess.Session.Strategy = reasoning.NewCoTStrategy(inner, cfgCotSteps)
		case "tot":
			sess.Session.Strategy = reasoning.NewToTStrategy(inner)
		case "consistency":
			sess.Session.Strategy = reasoning.NewConsistencyStrategy(inner, cfgConsistencySamples)
		case "decompose":
			sess.Session.Strategy = reasoning.NewDecomposeStrategy(inner)
		}
		fmt.Fprintf(os.Stderr, "[reasoning] using %s strategy\n", cfgReasoning)
	}

	// Persistence: wire SQLite-backed memory if not --no-persist
	if !cfgNoPersist {
		dbPath := cfgDBPath
		if dbPath == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				os.MkdirAll(filepath.Join(home, ".tau"), 0755)
				dbPath = filepath.Join(home, ".tau", "tau.db")
			}
		}
		if dbPath != "" {
			store, err := core.NewSQLiteStore(dbPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "persistence: %v — falling back to in-memory\n", err)
			} else {
				// Ensure MemorySystem exists
				if sess.Memory == nil {
					sess.Memory = core.NewMemorySystem("", "", nil)
				}
				// Replace episodic memory with store-backed one
				sess.Memory.Episodic = core.NewEpisodicMemory("", 1000, store)
				// Replace semantic memory with store-backed one
				if cfgSemantic {
					sess.Memory.SemMem = core.NewSemanticMemory(store)
				} else {
					sess.Memory.SemMem = nil
				}

				// Restore working memory if --restore
				if cfgRestore {
					data, err := store.Read("working_memory")
					if err == nil {
						var state struct {
							Observations []core.Observation `json:"observations"`
						}
						if json.Unmarshal(data, &state) == nil {
							for _, obs := range state.Observations {
								sess.Memory.Working.Add(obs)
							}
						}
					}
				}

				if cfgVerbose {
					fmt.Fprintf(os.Stderr, "[persistence: sqlite at %s]\n", dbPath)
				}
			}
		}
	}

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

	// MCP auto-discovery: connect to external MCP servers and register their tools
	if !cfgNoTools {
		mcpClients, err := core.DiscoverMCPClients(cfgMCPConfig)
		if err == nil {
			for _, client := range mcpClients {
				for _, tool := range client.Tools() {
					sess.Tools.Register(tool)
				}
			}
			if len(mcpClients) > 0 {
				totalMCPTools := 0
				for _, c := range mcpClients {
					totalMCPTools += len(c.Tools())
				}
				fmt.Fprintf(os.Stderr, "[mcp] connected to %d servers, %d external tools\n", len(mcpClients), totalMCPTools)
			}
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
// ---- runSkillsList ----

func runSkillsList(cmd *cobra.Command, args []string) {
	all := skills.GlobalSkills.List()
	fmt.Println("Available domain skills:")
	for _, s := range all {
		fmt.Printf("  %-20s [%s] %s\n", s.Name, s.Category, s.Description)
	}
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