# tau — Go Agent Runtime

A Go agent runtime kernel + provider abstraction. Minimum controllable runtime for LLM agents, with coding-agent product layer.

## Architecture

```
cmd/tau/              CLI (batch, TUI, WebUI)
pkg/coding/           Product: tools + skills + prompts + session
pkg/tui/              TUI: Bubble Tea
pkg/webui/            WebUI: HTTP + WebSocket
pkg/extensions/       goja JS VM extensions
pkg/provider/         Model registry + provider loading
pkg/persist/          Session persistence (JSONL)
pkg/sandbox/          Docker/Podman container sandbox
pkg/testing/faux/     Faux provider (zero API keys)
core/                 Kernel: Loop, Session, Transcript, Tool, Hook, AgentEvent
providers/            5 wires: OpenAI, Anthropic, Google, Azure, Mistral
toolspec/             Tool schema codegen
```

## Capabilities

| Layer | Count | Details |
|-------|-------|---------|
| Tools | 10 | read, write, edit, bash, glob, grep, task, task_tracker, web_search, web_fetch |
| Skills | 8 | code-review, debugger, test-writer, refactor, architect, go-refactor, shell-scripting, git-workflow |
| Wire protocols | 5 | OpenAI Completions, Anthropic Messages, Google GenAI, Azure OpenAI, Mistral |
| UI | 3 | Batch CLI, TUI (Bubble Tea), WebUI (HTTP+WebSocket) |
| Extensions | goja | JS VM runtime, 20+ events, hot-reload |
| Sessions | persist | JSONL save/resume/list, fork, CWD recording |
| Hooks | 8 | BeforeProviderRequest, TransformContext, Before/AfterToolCall, BeforeCompaction, BeforeAgentStart, AfterToolResult, ShouldStopAfterTurn |
| Compaction | ✓ | Token-threshold trigger, LLM summarization, iterative summary carry-over |
| Steer | ✓ | Mid-turn direction injection via SteerQueue |
| Event Bus | ✓ | 20 event types, subscribe/emit, thread-safe |
| Tests | 28+ | Faux-based E2E (zero API keys), unit + integration |
| Sandbox | Docker/Podman | Container isolation with path-jail fallback |

## Quick Start

```bash
# Build
go build -o tau ./cmd/tau/

# Smoke test (needs ANTHROPIC_AUTH_TOKEN for LLM; uses faux for tests)
echo "say hello" | ./tau --max-turns 1

# E2E tests (zero API keys)
go test -count=1 ./core/ ./pkg/testing/faux/ ./pkg/coding/tools/ ./pkg/persist/ ./pkg/extensions/ ./providers/...

# Multi-turn demo
go run ./demo/

# List capabilities
./tau --list-models
./tau --list-tools
./tau --list-sessions
```

## Project Stats

- **Go files**: ~40
- **Go lines**: ~9,600
- **Tests**: 28+ (all PASS)
- **Core interfaces**: Loop, Session, Transcript, Tool, Provider, AgentEvent, Hook, WireCompat
- **Design principles**: R1-R6 iron laws, zero `any` in public API, sealed interfaces

## Adding a Provider

1. Create `providers/<name>/provider.go` — implement `core.Provider` (Stream + Complete).
2. Add model to `pkg/provider/models.json`.
3. Register factory in `pkg/provider/lazy.go`.
4. Add `//go:build !no_<name>` constraint.

## Adding a Tool

1. Create `pkg/coding/tools/<name>.go` — return `core.Tool{Name, Description, Schema, Execute}`.
2. Register in `pkg/coding/session.go` → `SetActive` list.
3. Add test in `pkg/coding/tools/tools_e2e_test.go`.

## Adding a Skill

1. Create `pkg/coding/skills/builtin/<name>.md` — frontmatter + guidelines.
2. System prompt auto-discovers via `SystemPromptFn` closure.
