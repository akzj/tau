# 8. Cross-cutting Design Principles, Top 5 Borrowable Ideas, Surprises

## 8.1 Cross-cutting design principles

These are the patterns that recur throughout pi and are worth absorbing as architectural defaults for the Go port.

### Append-only beats mutable state

Session entries, leaf pointers, model changes, compactions — all are events. Recovery, fork, navigation, and undo become trivial. There is no "save the session" code path because every mutation is already on disk.

### Hooks at every meaningful boundary, with chain-of-responsibility patches

Before agent start, context transform, before/after provider request, before payload send, before/after tool call, before/after compaction, before/after tree navigation. The default behavior is a chain with no handlers; the harness composes hooks for storage, the application composes hooks for UI / extensions / policy. **Last-non-undefined-wins** semantics for hooks producing a single result.

The "patch returned, applied field-by-field, no deep merge" rule keeps semantics predictable.

### The wire protocol is the unit of pluggability for AI providers, not the vendor

Compat flags do per-vendor tuning inside one provider. ~30 vendors map to ~9 wire protocols.

### Phase machine prevents concurrency bugs without locks

A single enum (`idle | turn | compaction | branch_summary | retry`) blocks invalid transitions. Combined with `pendingSessionWrites`, mid-turn state changes are safe.

### Errors are values

Provider failures, tool failures, compaction failures all return `Result<T, E>` or end up encoded in messages. `throw` is reserved for true programmer errors. **The agent loop has almost no `try/catch`** — that's the design's signature.

### Streaming is the only mode

Non-streaming consumers `await stream.result()`. This eliminates the dual-path complexity that haunts most LLM libraries.

### Multi-session safety = no shared writable state, not locks

One file per session, append-only. No `flock`, no daemon, no inter-process coordination.

### Pinned dependencies, no auto-updates, lockfile-as-reviewed-code

External deps treated as supply-chain risk; lifecycle scripts blocked unless explicitly allowlisted. The `.npmrc` sets `save-exact=true` and `min-release-age=2`.

### "No backward compat unless asked"

Refactor freely. AGENTS.md states this explicitly. The codebase shows it: `version:3` JSONL header with no migration code in the repo.

### Provider usage is the source of truth for tokens/cost

Estimation is fallback only. Don't reimplement tokenizers — providers already do it and report back.

---

## 8.2 Top 5 borrowable ideas (ranked for a Go agent system)

### 1. Append-only session tree with `LeafEntry` pointers

**Single most reusable idea.** JSONL on disk, in-memory `byId` map, navigation-as-event. Free undo, free fork, free time-travel.

- ~300 LOC in Go for a clean implementation.
- See [02-harness.md §2.1–2.3](02-harness.md) for the data shape.

### 2. Iterative compaction with cut-point validation + previous-summary update + file-op tracking

Don't just "summarize when full." Snap cuts to user/assistant/bashExec boundaries, **never split a tool call from its result**, feed the previous summary as input to keep summaries stable across compactions, and accumulate `<read-files>` / `<modified-files>` blocks.

- ~600 LOC in Go, including the cut-point algorithm.
- See [02-harness.md §2.4](02-harness.md) for the algorithm.

### 3. Hook bus with chain-of-responsibility patches at every boundary

`before_agent_start`, `context`, `before_provider_request`, `before_provider_payload`, `tool_call`, `tool_result`, `session_before_compact`, `session_before_tree`. Lets you build the harness, then layer policy/UI/extensions without touching the core. The "patch returned, applied field-by-field, no deep merge" rule keeps semantics predictable.

### 4. Two-queue steering model

`steer` = inject between turns; `followUp` = inject after agent would stop. `QueueMode = "all" | "one-at-a-time"`. Maps cleanly onto interactive UX. The `nextTurn` queue (pre-prompt injection, before the next user prompt is sent) is also worth porting.

### 5. Faux provider that implements the real `StreamFunction` contract

Simulated streaming deltas + simulated prompt cache. Lets you test the *real* loop, the *real* harness, the *real* compaction, with deterministic outputs. **This is what makes pi's tests fast and meaningful.** No mock layer above the provider.

- See [07-testing-faux-provider.md](07-testing-faux-provider.md).

---

## 8.3 What pi does NOT do well (avoid replicating)

### Two queue layers

Both `Agent` (in `agent.ts`) and `AgentHarness` have their own steer/followUp queues, and the harness's queues delegate to the loop's via `getSteeringMessages` / `getFollowUpMessages`. There's a small duplication of `PendingMessageQueue` logic. **In Go, keep one queue layer (in the harness).**

### `AgentHarnessOwnEvent` and `AgentEvent` are siblings

The harness exposes an *unioned* event stream (loop events + harness events). Subscribers must switch on a wide enum. This works, but in Go a single `Event` interface with explicit type assertions keeps things tidier than a union.

### TS-specific `declare module` extension of `CustomAgentMessages`

Clever but invisible to Go. Replace with explicit registration:

```go
RegisterCustomMessageType("bashExecution", &BashMessageHandlers{
    Convert:        ...,
    EstimateTokens: ...,
})
```

### `prepareArguments` shim for tools

A band-aid for raw-string-args from older models. Not necessary if your tool args are always parsed JSON.

### Compaction prompt is hard-coded

Hard-coded with TS template strings inside `compaction.ts`. **Make it configurable in the Go port** (a `CompactionPolicy` interface with `BuildPrompt(messages, prevSummary) string`).

### `models.generated.ts` is 16K LOC included in the package

Recomputed regularly via a script. In Go, fetch the model registry dynamically or ship a small JSON file you can swap.

### Lazy provider loading via `import.meta.url` munging

Ugly for the Bun-binary case. Go can just blank-import all providers at init and rely on the linker to dead-strip unused ones (or use build tags to exclude expensive providers like Bedrock).

### Multi-session "safety" relies on convention

`AGENTS.md` is rules-for-the-agent, not code. **In Go, encode the rules in the bash/git tools themselves** (the `bash` tool refuses `git add -A`, the `git` tool only allows whitelisted subcommands).

---

## 8.4 Surprises (worth flagging)

### 1. No filesystem locking anywhere

I expected `lockfile` or `flock` for multi-session. Pi achieves safety entirely through file-per-session + append-only. Elegant, and probably correct for the single-writer-per-session invariant.

### 2. The agent loop has no max-turns or token-budget cap

Termination is purely "no more work + no more queued messages." All budget enforcement lives in `shouldStopAfterTurn` / `prepareNextTurn` hooks (i.e., the harness). Cleaner than mixing concerns into the loop.

### 3. The compaction summary entry is appended to the session like any other event

It's not a "side note"; future compactions read it as `previousSummary` and update it. **Compactions stack, summaries grow, history stays.**

### 4. `AgentHarness.compact()` allows hooks to *supply* the compaction

`session_before_compact` returning `{compaction}` skips the LLM call entirely. This means an extension could do compaction with a cheaper model, a local LLM, or a deterministic algorithm. Same trick for branch summaries.

### 5. `setLeafId` is itself an entry

`LeafEntry { type:"leaf", targetId }`. Reverting a navigation = appending another leaf entry. The current leaf is the targetId of the most recent leaf-bearing entry. **This is the cleanest "current branch pointer in an immutable log" implementation I've seen.**

### 6. The `compat` field carries 20+ vendor knobs

Pi has clearly hit every weird API in production: DeepSeek `thinking: { type }`, Together `reasoning: { enabled }`, ZAI `enable_thinking`, Qwen `chat_template_kwargs.enable_thinking`, Anthropic-on-Fireworks needing `cache_control` skipped on tools. **Plan for ugly compat flags — they will accrue.**

### 7. `Usage` separates `cacheRead`/`cacheWrite` from `input`/`output`

Pi explicitly tracks prompt-cache hits because they're 10× cheaper. Cost calculation treats all four separately (`model.cost.cacheRead`, `model.cost.cacheWrite`).

### 8. `getApiKey` is per-call, not per-session

Designed for short-lived OAuth tokens (GitHub Copilot, OpenAI Codex). Tools can run for minutes; the token at agent-start may expire before the next provider call. **Always re-resolve.**