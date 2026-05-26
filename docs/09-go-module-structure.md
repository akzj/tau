# 9. Recommended Go Module Structure

This is a starting layout for porting pi's design to Go. Sizes are rough estimates informed by pi's LOC counts, scaled for Go's verbosity (~+30%).

## 9.1 The layout

```
goagent/
├── core/                           # ~2K LOC
│   ├── agent_loop.go               # runLoop, executeToolCalls (sequential/parallel)
│   ├── agent_loop_continue.go      # continuation entry
│   ├── events.go                   # AgentEvent union, EventSink type
│   ├── types.go                    # AgentMessage, Tool, ToolResult, AgentContext
│   └── queue.go                    # PendingMessageQueue (one-at-a-time / all)
│
├── harness/                        # ~3K LOC
│   ├── harness.go                  # AgentHarness: phase machine, prompt/steer/followUp/compact
│   ├── hooks.go                    # typed hook bus + broadcast subscribers
│   ├── stream_options.go           # snapshot + patch semantics
│   ├── system_prompt.go            # dynamic system prompt + skills XML block
│   ├── messages.go                 # AgentMessage <-> LLM Message conversion
│   └── compaction/
│       ├── compaction.go           # shouldCompact, prepareCompaction, compact
│       ├── cut_point.go            # findCutPoint, findTurnStartIndex
│       ├── prompts.go              # SUMMARIZATION_PROMPT, UPDATE_, TURN_PREFIX_
│       ├── file_ops.go             # FileOperations, extractFromMessage, formatXMLBlocks
│       └── branch_summary.go       # collectEntriesForBranchSummary, generateBranchSummary
│
├── session/                        # ~1.5K LOC
│   ├── session.go                  # Session: append APIs (message/model_change/etc)
│   ├── tree.go                     # SessionTreeEntry union, buildSessionContext
│   ├── jsonl_storage.go            # JsonlSessionStorage (the canonical impl)
│   ├── jsonl_repo.go               # create/open/list/delete/fork
│   ├── memory_storage.go           # MemorySessionStorage (tests)
│   └── uuidv7.go                   # 8-char UUIDv7 prefixes with collision retry
│
├── tools/                          # ~500 LOC framework + per-tool
│   ├── tool.go                     # Tool interface, ToolResult, ExecutionMode
│   ├── builtins/
│   │   ├── read.go
│   │   ├── write.go
│   │   ├── edit.go
│   │   ├── bash.go                 # NOTE: enforces multi-session git rules here
│   │   ├── grep.go
│   │   └── ls.go
│   └── truncate.go                 # shell-output truncation w/ fullOutputPath
│
├── providers/                      # ONE FILE PER WIRE PROTOCOL, not per vendor
│   ├── registry.go                 # Provider, ProviderRegistry, sourceID for unregister
│   ├── stream.go                   # streamSimple, completeSimple
│   ├── event_stream.go             # typed channel-based EventStream
│   │
│   ├── faux/                       # THE testing tool; ship in same module
│   │   ├── faux.go
│   │   └── prompt_cache.go         # common-prefix simulation
│   │
│   ├── openai_completions/         # largest provider; covers ~30 vendors via compat
│   │   ├── openai_completions.go
│   │   ├── compat.go               # OpenAICompletionsCompat with all flags
│   │   ├── transform_messages.go
│   │   └── thinking_format.go      # openai/openrouter/deepseek/together/zai/qwen
│   │
│   ├── anthropic_messages/
│   ├── openai_responses/
│   ├── google_genai/
│   ├── google_vertex/
│   ├── bedrock_converse/           # build-tag opt-in (large AWS SDK dep)
│   └── mistral/
│
├── env/                            # ExecutionEnv: FileSystem + Shell
│   ├── env.go                      # interfaces, Result types, error codes
│   ├── filesystem_os.go            # os.* implementation
│   └── shell_exec.go               # os/exec implementation w/ abort + timeout
│
├── skills/                         # SKILL.md + prompt-template loaders
│   ├── skills.go                   # loadSkills, formatSkillsForSystemPrompt
│   ├── prompt_templates.go         # loadPromptTemplates, $1/$@/$ARGUMENTS
│   └── frontmatter.go              # YAML frontmatter parse
│
└── extensions/                     # OPTIONAL — only if you need plug-in story
    ├── api.go                      # ExtensionAPI host object
    ├── runtime.go                  # invalidate-and-throw stale-ctx semantics
    └── runner_starlark.go          # recommended embedded scripting backend
        runner_goja.go              # alternative: JS/TS-flavored
```

### Target sizes

| Module | LOC | Notes |
|--------|------|-------|
| `core/` | ~2K | Pure agent loop + event types |
| `harness/` | ~3K | Includes compaction subdir |
| `session/` | ~1.5K | Pure storage |
| `tools/` (framework) | ~500 | Excludes builtins |
| `providers/` (per wire protocol) | ~3K each | Mostly auth + payload mapping |
| `providers/faux/` | ~600 | Critical for testing |
| `env/`, `skills/` | ~500 each | Small utilities |

**Total agent runtime in the 12–15K LOC ballpark**, plus per-vendor extensions.

## 9.2 Build order

Each layer is testable in isolation against the next-lower layer. Build in this order:

### 1. `session/`

Verifiable in isolation with simple unit tests (no LLM, no harness).

- Create / append / read entries.
- Walk path-to-root.
- `setLeafId` correctness.
- Forking entry-slice copy.
- Round-trip JSONL serialization.

### 2. `providers/faux/` + `providers/registry.go` + `providers/event_stream.go`

Verifiable in isolation.

- Queue-based response emission.
- Streaming deltas.
- Prompt-cache simulation (common-prefix).
- Abort propagation.
- `onResponse` / `onPayload` invocation.

### 3. `core/` (agent loop) using only the faux provider

**This is where 90% of the agent's behavior is testable.**

- Single-turn happy path.
- Multi-turn with tool calls (sequential + parallel).
- Steering injection between turns.
- Follow-up injection after stop.
- Abort during streaming.
- Tool error → result with `isError: true`.
- `terminate: true` from all tools → loop exits.
- `shouldStopAfterTurn` → loop exits.

### 4. `harness/` connecting session + loop + hooks

Adds compaction and lifecycle.

- Phase machine: idle → turn → idle.
- `pendingSessionWrites` flushed at correct boundaries.
- Hook bus: chain-of-responsibility, last-wins.
- Subscribe vs `on(eventType)`: both fire correctly.
- Custom system prompt function invoked per turn.
- `setActiveTools` toggling without rebuilding catalog.

### 5. `harness/compaction/`

Port the algorithm and prompts; test against canned multi-turn faux sessions.

- `shouldCompact` triggers correctly given simulated `Usage`.
- `findCutPoint` snaps to valid boundaries.
- `findCutPoint` never splits a `toolCall` from its `toolResult`.
- Iterative compaction: previous summary fed in correctly.
- Split-turn case produces two-part summary.
- File op accumulation across compactions.
- `session_before_compact` hook can supply the compaction (skipping LLM call).

### 6. `tools/` and `env/`

Implement only the surface area you actually need. Likely starting set:

- `read`, `write`, `edit` (file ops with `path` arg → file-op tracking).
- `bash` (with banned-command list for multi-session safety).
- `grep`, `ls` (read-only).

### 7. Real providers, one wire-protocol at a time

Start with `openai_completions` (highest leverage — covers ~30 vendors via `compat` flags).

Then add `anthropic_messages`, `openai_responses`, `google_genai` as needed.

### 8. `extensions/` if and only if you actually need user-extensible runtime behavior

If you don't need third-party-author extensions, skip this entirely. Most of pi's flexibility is already exposed via the hook bus.

## 9.3 Why this ordering

**This ordering lets you validate the entire core+harness+compaction stack against the faux provider before writing a single byte of vendor-specific HTTP code.**

The faux provider is the leverage point. Once it works, every layer above it is testable without network, API keys, or paid tokens. Vendor-specific code is then *just* HTTP/SDK glue — the *behavior* of the agent is already correct.

This is the inverse of the usual "let me hook up Anthropic first to see something work" approach, which couples your agent code to vendor quirks before you've nailed the abstractions.

## 9.4 Suggested first milestone

Define "v0.1 minimum viable" as:

- `session/` works (round-trip JSONL).
- `providers/faux/` works (queue + streaming + prompt cache).
- `core/agent_loop` works against faux.
- `harness/` glues them with no compaction yet, no hook bus yet — just: prompt → tool calls → tool results → another prompt.
- One real provider: `openai_completions` (so you can run against any OpenAI-compatible endpoint, including local llama.cpp).
- Two builtin tools: `read` + `bash`.

That's enough to:
- Run a real LLM session.
- Validate the faux-provider tests against a real model (sanity check).
- Have a foundation for layer 4+ (compaction, hooks, extensions).

Estimated size of v0.1: ~5K LOC. Estimated time: depends on developer familiarity with HTTP streaming + Go generics. Plan for the openai-completions provider alone to be 1.5–2K LOC once you handle thinking, tool calls, prompt caching, and the compat flags you actually need.