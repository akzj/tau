# tau Architecture

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        INTERFACES                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────────┐  │
│  │   TUI    │  │  WebUI   │  │   CLI (batch + plugin)   │  │
│  │Bubble Tea│  │HTTP+WS   │  │   tau [args] <prompt>    │  │
│  └────┬─────┘  └────┬─────┘  └────────────┬─────────────┘  │
│       │             │                     │                 │
├───────┴─────────────┴─────────────────────┴─────────────────┤
│                    AGENT LOOP (core/)                       │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Loop: Prompt → Continue → (tool_calls → Continue)* │   │
│  │  Session: Transcript + Tools + Providers + Hooks     │   │
│  │  AgentEvent: sealed interface (9 types)              │   │
│  │  EventBus: 20+ event types, subscribe/emit           │   │
│  │  Retry: exponential backoff + jitter (429/503/timeout)│  │
│  └──────────────────────────────────────────────────────┘   │
│       │                                                     │
│  ┌────┴────────────────────────────────────────────────┐    │
│  │  SKILLS (30: 20 general + 10 domain)                │    │
│  │  SystemPromptFn closure → auto-inject               │    │
│  │  3-layer discovery: builtin → user → project        │    │
│  └─────────────────────────────────────────────────────┘    │
│       │                                                     │
│  ┌────┴────────────────────────────────────────────────┐    │
│  │  TOOLS (20)                                          │    │
│  │  read/write/edit/bash/glob/grep/task/...             │    │
│  │  Three-phase: Prepare → Execute → Finalize           │    │
│  │  Details/Content separation for structured output    │    │
│  └─────────────────────────────────────────────────────┘    │
│       │                                                     │
│  ┌────┴────────────────────────────────────────────────┐    │
│  │  PLUGINS (external + in-process)                    │    │
│  │  JSON-RPC over stdio, plugin CLI (list/install/...)  │    │
│  └─────────────────────────────────────────────────────┘    │
│       │                                                     │
│  ┌────┴────────────────────────────────────────────────┐    │
│  │  MULTI-AGENT                                         │    │
│  │  SubAgentPool: Spawn → Collect (WaitGroup)           │    │
│  │  Sandbox: command whitelist + network isolation      │    │
│  └─────────────────────────────────────────────────────┘    │
│       │                                                     │
├───────┴─────────────────────────────────────────────────────┤
│                    PROVIDERS (9 wires)                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ OpenAI   │ │Anthropic │ │ Google   │ │ Azure OpenAI  │  │
│  │Completions│ │Messages  │ │  GenAI   │ │               │  │
│  ├──────────┤ ├──────────┤ ├──────────┤ ├───────────────┤  │
│  │ Mistral  │ │ Bedrock  │ │ Vertex AI│ │    Codex      │  │
│  │          │ │  (AWS)   │ │  (GCP)   │ │  Responses    │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
│              OpenAI Responses + Codex Responses             │
├─────────────────────────────────────────────────────────────┤
│                    INFRASTRUCTURE                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ Config   │ │ Session  │ │ Sandbox  │ │ Extensions    │  │
│  │tau.yaml  │ │Persist   │ │Docker    │ │ goja JS VM    │  │
│  ├──────────┤ ├──────────┤ ├──────────┤ ├───────────────┤  │
│  │ Health   │ │ Metrics  │ │ Logging  │ │ Shutdown      │  │
│  │/health   │ │Prometheus│ │ log/slog │ │ SIGINT/TERM   │  │
│  ├──────────┤ ├──────────┤ ├──────────┤ ├───────────────┤  │
│  │ Docker   │ │ CI/CD    │ │ Version  │ │ Faux Provider │  │
│  │Container │ │GitHub Act│ │ ldflags  │ │ zero API key  │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Layer Description

### Interfaces (TUI / WebUI / CLI)
- **TUI**: Bubble Tea terminal UI with syntax highlighting, file tree sidebar, keyboard shortcuts
- **WebUI**: HTTP + WebSocket server, dark theme, responsive, session management
- **CLI**: Batch mode, `--tui`, `--webui`, `--resume`, `--list-*`, `plugin` subcommand

### Agent Loop (core/)
- **Loop**: `Prompt()` → LLM response → tool calls → `Continue()` cycle
- **Session**: Unit of isolation. Transcript + ToolRegistry + ProviderRegistry + HookSet
- **AgentEvent**: Sealed interface (9 concrete types). Product layer consumes via `Run.Events`
- **EventBus**: 20+ event types, subscribe/emit, thread-safe
- **Retry**: Exponential backoff + jitter for transient errors (429, 503, timeout, connection reset, EOF, broken pipe, connection refused). Config: 3 retries, 1s initial delay, 30s max, ±10% jitter.

### Skills (30 total: 20 general + 10 domain)
- Injected via `SystemPromptFn` closure in system prompt
- 3-layer discovery: builtin → user (`~/.tau/skills/`) → project (`.tau/skills/`)
- Markdown files with frontmatter: `name`, `description`, `when_to_use`, `steps`
- 20 general: code-review, debugger, test-writer, refactor, architect, go-refactor, shell-scripting, git-workflow, go-code-review, go-debugging, go-test-writing, go-refactoring, python-code-review, python-debugging, python-test-writing, python-refactoring, js-code-review, js-debugging, js-test-writing, js-refactoring
- 10 domain: refactoring-patterns, debugging-strategies, api-design, database-patterns, cicd-patterns, testing-strategy, security-review, code-review-intensive, performance-optimization, documentation-generation

### Tools (20)
- All `core.Tool` values with `Execute` closure. Registered in `pkg/coding/session.go` via `cs.Tools.Register()`.
- Three-phase protocol: Prepare → Execute → Finalize
- Sandbox: path-jail for file tools, env-allowlist for bash, container routing
- Details/Content separation: Content (LLM text), Details (structured: path, size, exit_code)
- Full list: `read`, `write`, `edit`, `bash`, `glob`, `grep`, `task`, `task_tracker`, `web_search`, `web_fetch`, `workspace_diag`, `list_files`, `search_code`, `run_tests`, `git_diff`, `ask_user`, `lint`, `format`, `deps`, `coverage`

### Plugins (external + in-process)
- External: JSON-RPC over stdio, `Discover()` scans `$TAU_PLUGIN_DIR`
- In-process: `Plugin` interface with `Name()/Version()/Tools()/Providers()`
- CLI: `tau plugin list|install|remove|info`

### Multi-Agent
- `SubAgentPool`: Spawn → Collect with `WaitGroup`
- `SandboxSpec`: command whitelist, network isolation (CLONE_NEWNET), path sandboxing
- `spawn_agent` tool: programmatic sub-agent spawning

### Providers (9 wires)
- OpenAI Completions, Anthropic Messages, Google GenAI, Azure OpenAI
- Mistral, Bedrock (AWS), Vertex AI (GCP)
- OpenAI Responses, Codex Responses
- ModelRegistry: `models.json` with contextWindow/cost/inputTypes
- ProviderLoader: lazy-load with `sync.Once` caching
- All providers automatically benefit from retry logic via `core/loop.go`

### Infrastructure
- **Config**: `tau.yaml` (flag > env > config > default)
- **Session Persistence**: JSONL save/resume/list (`~/.tau/sessions/`)
- **Sandbox**: Docker/Podman container isolation with path-jail fallback
- **Extensions**: goja JS VM, 20+ events, hot-reload (fsnotify)
- **Health**: `/health` (JSON) + `/ready` (200)
- **Metrics**: Prometheus text format (tau_turns_total, tau_tool_calls_total, etc.)
- **Logging**: `log/slog` (zero deps)
- **Shutdown**: SIGINT/SIGTERM, 10s deadline, session auto-save
- **Docker**: multi-stage Dockerfile + docker-compose.yml
- **CI/CD**: lint/test/bench/build (Go 1.22+1.23), goreleaser
- **Faux Provider**: zero API key testing infrastructure — all tests pass without real LLM credentials

## Data Flow

```
User Input → CLI/TUI/WebUI → Loop.Prompt(session, input)
  → SystemPromptFn (injects skills)
  → Provider.Stream (LLM API call via wire)
  → SSE Parser → ProviderEvent → Loop.processProviderEvents
    → AgentEvent stream → TUI/WebUI rendering
    → Tool calls → Tool.Execute (parallel via WaitGroup)
    → ToolResult → Transcript.Append
  → Continue() → (repeat until no tool_calls)
  → Session persistence (JSONL save)
```

## Extension Points

| Point | How to Extend |
|-------|--------------|
| Add a tool | Create `pkg/coding/tools/<name>.go`, return `core.Tool`, register in `session.go` |
| Add a provider | Create `providers/<name>/provider.go`, implement `core.Provider`, add to `lazy.go` |
| Add a skill | Create `pkg/coding/skills/builtin/<name>.md` with frontmatter |
| Add a hook | Add handler to `HookSet` in `session.go`: `sess.Hooks.BeforeToolCall.Add(...)` |
| Add an extension | Create `.js` file in `.tau/extensions/`, use `tau.on(event, handler)` |
| Add a plugin | Create binary implementing JSON-RPC stdin/stdout, put in `.tau/plugins/` |