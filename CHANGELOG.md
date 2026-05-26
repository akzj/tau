# Changelog

## [Unreleased]

### Features — Wire Protocols (9/9)
- OpenAI Completions — SSE streaming, multi-turn Loop, gpt-5.4/gpt-4o/o1 models
- Anthropic Messages — extended thinking, tool_use blocks, claude-sonnet-4-6/haiku-3-5/opus-4
- Google GenAI — Gemini SSE, functionCall parsing, gemini-2.5-flash/pro
- Azure OpenAI — api-key auth, api-version query param, azure-gpt-4o
- Mistral — Bearer auth, OpenAI-compatible, mistral-large/small/codestral
- Bedrock (AWS) — SigV4 signing, JSON Lines streaming, Claude 3.5 Sonnet/Haiku
- Vertex AI — OAuth2 Bearer, gcloud ADC fallback, vertex-gemini-2.5-flash/pro
- OpenAI Responses — `/v1/responses`, `response.created/delta/completed` SSE events, gpt-4o-responses/o3-mini
- Codex Responses — full SSE streaming, `CODEX_ORG_ID`, OpenAI-Organization header

### Features — Tools (16)
- read / write / edit — path-jail sandbox, backup, binary detection, param completeness
- bash — env allowlist, git safety, timeout/signal/exit-code, container routing
- glob / grep — `**` walk, regex context_lines, ignore_case, binary skip
- task / task_tracker — session-scoped CRUD, status workflow (pending→in_progress→completed→cancelled)
- web_search — DuckDuckGo Instant Answer (zero API keys)
- web_fetch — HTTP GET, HTML→text strip, 10s timeout, scheme validation
- workspace_diag — file tree scan, git status, language statistics, extension breakdown
- list_files — directory listing, depth/glob/hidden filter, size+type display
- search_code — regex search with context lines, file_glob filter
- run_tests — auto-detect framework (go/py/js/rust), 120s timeout
- git_diff — git diff with --staged support, structured output
- ask_user — pause loop with Terminate:true, options support

### Features — Skills (20)
- 8 general: code-review, debugger, test-writer, refactor, architect, go-refactor, shell-scripting, git-workflow
- 4 Go: go-code-review, go-debugging, go-test-writing, go-refactoring
- 4 Python: python-code-review, python-debugging, python-test-writing, python-refactoring
- 4 JS/TS: js-code-review, js-debugging, js-test-writing, js-refactoring
- 3-layer discovery: builtin → user (~/.tau/skills/) → project (.tau/skills/)

### Features — TUI (Bubble Tea)
- Terminal UI with streaming message display, color theme (6 styles), status bar
- File tree sidebar (Ctrl+T), color-coded (green ●=created, yellow ○=modified, grey ·=read)
- Syntax highlighting: Go/Python/JS/Bash code blocks, keywords + strings + comments
- Keyboard: Ctrl+C quit, Ctrl+L clear, Ctrl+S save, Ctrl+R resume, Ctrl+N new, Ctrl+P provider, Ctrl+M model
- Mouse wheel scrolling, turn counter, session persist integration

### Features — WebUI (HTTP + WebSocket)
- Dark theme chat UI with streaming message display, color-coded message types
- File tree sidebar (📁 Files toggle), tool collapse/expand (click-to-toggle)
- Session management: new session, resume from persist, session list dropdown
- Responsive design: tablet (≤768px) + mobile (≤480px) breakpoints, 44px touch targets
- `/health` (JSON status) + `/ready` (200) endpoints

### Features — Infrastructure
- **Configuration**: `tau.yaml` (YAML, flag > env > config > default), `--config` flag
- **Session Persistence**: JSONL save/resume/list (`~/.tau/sessions/`), atomic write, corrupt-line skip, CWD recording
- **Compaction**: token-threshold trigger, LLM summarization (Provider.Complete), iterative summary carry-over, file op extraction, BeforeCompaction hook
- **Container Sandbox**: Docker/Podman with alpine:latest, --network none, --memory 512m, path-jail fallback
- **Extension System**: goja JS VM runtime, 20+ event types, hot-reload (fsnotify), stale-context safety, pre-bind queueing
- **Graceful Shutdown**: SIGINT/SIGTERM, 10s deadline, session auto-save, second signal force-exit
- **Health Check**: `/health` (version, uptime, providers, tools) + `/ready` endpoints
- **Prometheus Metrics**: tau_turns_total, tau_tool_calls_total, tau_compact_operations_total, tau_session_duration_seconds (manual text format, zero deps)
- **Structured Logging**: log/slog (zero deps), --log-level + --log-format flags, 8 instrumented points
- **Phase Machine Guard**: ErrTurnInProgress mutex, concurrent Prompt/Continue rejection
- **Provider Lazy-Load**: sync.Once caching, ModelRegistry with contextWindow/cost/inputTypes, `--list-models`
- **Steer/FollowUp**: mid-turn direction injection via SteerQueue, `--steer` flag
- **Retry Logic**: MaxRetries/RetryDelay on StreamRequest, exponential backoff, transient-only
- **CLI**: batch mode, `--tui`, `--webui`, `--resume`, `--list-sessions`, `--list-models`, `--list-tools`, `--list-skills`
- **Docker**: multi-stage Dockerfile (golang:1.23→alpine:3.20), HEALTHCHECK, docker-compose.yml, .dockerignore
- **CI/CD**: ci.yml (lint/test/bench/build, Go 1.22+1.23), release.yml (goreleaser), .goreleaser.yml (linux/darwin/windows, amd64/arm64)

### Features — Core
- Loop closed-loop (Prompt→tool_call→Continue→final), parallel tool scheduling (WaitGroup), sequential tool ordering
- AgentEvent sealed (9 types), ProviderEvent (7 types), double-layer stream, EventBus (20 types)
- HookSet (8 hooks): BeforeProviderRequest, TransformContext, Before/AfterToolCall, BeforeCompaction, BeforeAgentStart, AfterToolResult, ShouldStopAfterTurn, BeforeSessionTree
- Chain[T]/LastWins[T] generics, ToolResultPatch, ThreePhaseTool (Prepare→Execute→Finalize), ToolSchema codegen
- PendingWrites tracking, per-tool parallel/sequential override, dynamic tool activation (SetTools/GetActiveTools)
- Session tree (5 entry types), Fork operation, ToolResult Details/Content separation, iterative compaction
- 6 iron laws (R1-R6): sealed interfaces, zero `any` in public API, Session-scoped registries, zero product names in core

### Tests (46+)
- 34 tool tests (edge cases: binary, empty, timeout, injection, invalid regex, cancelled status)
- 7 benchmarks (LoopPromptTurn 31µs, ToolExecution 237ns, Save 74µs, SSE Parse 291µs)
- Faux Provider: pre-queued response sequences, cache simulation, delta streaming, abort propagation, 7 tests
- Core E2E: Prompt→Continue cycle, compaction trigger, hook verification, phase guard, steer queue, parallel ordering
- Persist E2E: Save/Load roundtrip, corrupt line recovery, atomic write, CWD
- Extension E2E: multi-turn events, stale context fork, 8 tests

### Quality (27+ fixes)
- P0 fixes: nil map init, git safety bypass, lazy once.Do error cache, WebSocket WriteJSON mutex, JS RunProgram panic recovery, SSE scanner.Err() → ProvError
- P1 fixes: 9 patches (read error msg, edit WriteFile check, JSONL corrupt skip, atomic write, webui concurrency+shutdown, SSE json error, YAML check)
- P2 cleanup: 8 items (dead code removal, shared retry helper, error codes expansion, skills warnings, schema Validate, glob skipped list, registry duplicate warn, provider docs)
- Compat real flag fields (29 bools across 3 types), provider build tags (!no_openai/anthropic/google)
- ToolResult ordering guarantee verified + ToolResultPatch type added
- 6 json tag bugs found and fixed in toolThreePhase execute-closure args structs

### Documentation
- AGENTS.md (84 lines): architecture tree, 12-row capability table, quick start, contribution guides
- CONTRIBUTING.md (112 lines): fork→PR flow, adding tool/provider/skill, code style, testing, design principles
- CHANGELOG.md (this file)
- Comprehensive godoc: all exported symbols across 5 packages (core +65 lines)
- Tool parameter docs: 10 tools with type/default/constraint/example (67 lines)
- docs/v2/: 11 design documents (1,130 lines), docs/v2/10-tui.md, docs/v2/11-extension-system.md
- `tau.yaml` example config (41 lines)
- `.goreleaser.yml` release config
