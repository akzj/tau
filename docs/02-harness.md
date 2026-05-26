# 2. Harness (the most important part)

Source: `packages/agent/src/harness/agent-harness.ts` (995 LOC), `packages/agent/src/harness/types.ts` (815 LOC), `packages/agent/src/harness/compaction/compaction.ts` (755 LOC), `packages/agent/src/harness/session/session.ts` (252 LOC), `packages/agent/src/harness/session/jsonl-storage.ts` (293 LOC).

The `AgentHarness` is the layer above the agent loop that owns: **session log**, **compaction**, **branch summarization**, **hook bus**, **provider auth**, and **stream-option snapshotting**. It is the contract application code (e.g., the coding agent) writes against.

---

## 2.1 Append-only session tree (key data structure)

Sessions are not "list of messages" — they are a **tree of typed entries** stored as JSONL, one entry per line:

```
SessionTreeEntry =
  | MessageEntry            (user/assistant/toolResult/custom message)
  | ThinkingLevelChangeEntry
  | ModelChangeEntry
  | CompactionEntry         (summary + firstKeptEntryId + tokensBefore)
  | BranchSummaryEntry      (summary of an abandoned branch)
  | CustomEntry             (typed app data, NOT a message)
  | CustomMessageEntry      (typed app message, IS a message)
  | LabelEntry              (named pointer to another entry)
  | SessionInfoEntry        (session name)
  | LeafEntry               (records the active leaf pointer change)
```

Every entry has `{id, parentId, timestamp}`. The "current conversation" = the path from root to the active **leaf entry's targetId**. `getPathToRoot(leafId)` walks back via `parentId` in O(depth).

---

## 2.2 State management — minimal and recomputable

The harness's runtime state:

- `phase`: `"idle" | "turn" | "compaction" | "branch_summary" | "retry"` — **single state machine**, all entry-points assert `idle` before starting. **Prevents concurrent compaction-during-turn or fork-during-compaction without locks.**
- `pendingSessionWrites`: writes deferred during a turn, flushed on `turn_end` and at `prepareNextTurn` (so the next turn sees them).
- Three queues: `steerQueue`, `followUpQueue`, `nextTurnQueue` (pre-prompt injection). Each has `QueueMode = "all" | "one-at-a-time"`.
- `runAbortController`, `runPromise` for abort + idle-wait.
- `model`, `thinkingLevel`, `tools` map, `activeToolNames`, `resources` (skills + prompt templates), `streamOptions`.

**Everything else (context, message list) is derived from session via `session.buildContext()`.** The harness has no shadow copy of the conversation.

---

## 2.3 Storage — JSONL append-only

`JsonlSessionStorage` is the canonical implementation:

- **First line**: a versioned `SessionHeader { type:"session", version:3, id, timestamp, cwd, parentSession? }`.
- **Every other line**: a `SessionTreeEntry` JSON.
- All mutations are `appendFile` calls — no rewrites, no fsync gymnastics, no locks.
- An in-memory `byId` map and `labelsById` cache are rebuilt from the file on `open`.
- IDs are **8-char prefixes of UUIDv7** with collision retry (sortable by time, short for display, low collision risk). See `harness/session/uuid.ts`.
- **`setLeafId(id)` doesn't mutate state directly — it appends a `LeafEntry`.** The "current state" is whatever the **last `LeafEntry` (or appended entry)** points to. **Navigation is itself an event.**

This is the cleanest "current branch pointer in an immutable log" implementation I've seen.

There's also `MemorySessionStorage` (`Map`-backed) with the same interface for tests.

### Session file paths

Sessions are stored under `<sessionsRoot>/--<encoded-cwd>--/<timestamp>_<uuid>.jsonl`. The directory is keyed by encoded cwd, so per-cwd listing is O(dir-listing).

`SessionRepo.fork(source, {entryId, position})` copies the entry slice up to a point into a new file with a new header and `parentSessionPath` recorded.

---

## 2.4 Compaction — THE technique

This is the most reusable piece of pi. It lives in `compaction/compaction.ts`.

### When to compact

```typescript
shouldCompact(contextTokens, contextWindow, settings) =>
    contextTokens > contextWindow - settings.reserveTokens
```

Defaults (`DEFAULT_COMPACTION_SETTINGS`):
- `reserveTokens = 16384`
- `keepRecentTokens = 20000`

Tokens come from the **last successful assistant message's `usage.totalTokens`** plus an estimate (4 chars/token, with a +4800 char penalty for image content) for messages after it. **Provider usage is the source of truth; estimation is fallback only.**

### How to prepare compaction

`prepareCompaction(pathEntries, settings)`:

1. **Walk back** to find the **previous compaction entry**. Its `summary` becomes `previousSummary` for iterative update; its `firstKeptEntryId` becomes the new `boundaryStart`. **Compactions stack**: each one summarizes "everything since the last compaction" and is *fed the previous summary* via `UPDATE_SUMMARIZATION_PROMPT` so existing structure is preserved.

2. **`findCutPoint`**: scan from the end accumulating tokens until `keepRecentTokens` is reached. Then snap the cut to a **valid cut point** — a `user` / `assistant` / `bashExecution` / `branch_summary` / `custom_message` boundary. **Never cut between a `toolCall` and its `toolResult`** (this produces an LLM-rejected message sequence).

3. If the cut lands inside an in-progress assistant turn, mark `isSplitTurn` and find the turn's start. The "prefix" of the split turn is summarized **separately** with a different prompt (`TURN_PREFIX_SUMMARIZATION_PROMPT`) and concatenated with `\n\n---\n\n**Turn Context (split turn):**\n\n`.

4. Extract `FileOperations` (read/written/edited paths) from all summarized assistant tool calls + the previous compaction's details (so file lists also accumulate across compactions).

### Generating the summary

`generateSummary` calls the **same model** as the agent (`completeSimple`), with a strict structured prompt requiring sections:

- **## Goal**
- **## Constraints & Preferences**
- **## Progress** (Done / In Progress / Blocked)
- **## Key Decisions**
- **## Next Steps**
- **## Critical Context**

The update prompt says: **PRESERVE everything in `<previous-summary>`, only ADD/UPDATE.** The summary is appended with `<read-files>` / `<modified-files>` XML blocks listing all paths touched.

The system prompt for summarization is also explicit:

> "Do NOT continue the conversation. Do NOT respond to any questions in the conversation. ONLY output the structured summary."

### Persisting

A `CompactionEntry` is appended to the session: `{summary, firstKeptEntryId, tokensBefore, details:{readFiles, modifiedFiles}}`.

Then `buildSessionContext(entries)` re-builds the in-memory message list:

- Find the last compaction entry.
- Emit `[CompactionSummaryMessage(summary), ...entries from firstKeptEntryId onwards]`.
- The compaction summary is converted to a `user` message wrapped in `<summary>...</summary>` tags via `COMPACTION_SUMMARY_PREFIX/SUFFIX` (so the model sees it as a system-context handoff, not as an instruction).

**Pre-compaction history is NOT deleted** from the JSONL — it stays on disk, just not in the rebuilt context. Reverting compaction = appending a different leaf entry pointing before the compaction. **History is immutable; views are mutable.**

---

## 2.5 Truncation logic

Separate from compaction. `harness/utils/truncate.ts` (344 LOC) is an output-truncator (not in-prompt). When tool results are very long (e.g., shell output, file reads), it truncates the *displayed text* with `[... N more characters truncated]` and optionally saves full output to a temp file (`fullOutputPath`). It does **not** truncate by token count; it's a UX/DX concern.

`computeFileLists` and `formatFileOperations` in `compaction/utils.ts` produce the `<read-files>` / `<modified-files>` blocks that get appended to summaries.

---

## 2.6 System prompt assembly

`AgentHarnessOptions.systemPrompt` can be a string **or an async function** that receives `{env, session, model, thinkingLevel, activeTools, resources}`. The harness invokes it once per turn (`createTurnState`), so the prompt can be **dynamic** (e.g., include current cwd, current model, current set of active skills).

`formatSkillsForSystemPrompt(skills)` produces an XML `<available_skills>` block listing skill name / description / location for the model.

Skills (loaded from `SKILL.md` files with YAML frontmatter) and prompt templates (loaded from `.md` files) are **just text catalogs** with no executable code. The model invokes them by referring to their declared name; the harness has explicit `harness.skill(name)` and `harness.promptFromTemplate(name, args)` entry points that prepend the skill body / substitute `$1`, `$@`, `$ARGUMENTS` into the next user prompt.

---

## 2.7 Hook bus (the secret weapon)

The harness has **two** subscription mechanisms:

1. **`subscribe(listener)`** — receives every event (loop events + harness events). Used for UI rendering.

2. **`on(eventType, handler)`** — typed hook handlers that may **return values** that influence behavior:

| Hook event | What it can do |
|------------|----------------|
| `before_agent_start` | Replace `messages` and `systemPrompt` |
| `context` | Replace the whole transformed message list before the LLM call |
| `before_provider_request` | Patch headers/timeout/maxRetries/metadata/cacheRetention/transport (`undefined` deletes keys) |
| `before_provider_payload` | Rewrite the raw provider request body |
| `tool_call` | `block` a tool with a reason (becomes an error tool result) |
| `tool_result` | Patch content/details/isError/terminate |
| `session_before_compact` | `cancel`, or supply a pre-made compaction (skip the LLM call) |
| `session_before_tree` | Cancel a tree navigation, or supply a pre-made branch summary |

This is the **chain-of-responsibility pattern at every meaningful boundary**. Multiple handlers compose; the last non-undefined return wins for hooks that produce a single result.

---

## 2.8 Go sketch

```go
type AgentHarness struct {
    Env        ExecEnv         // FileSystem + Shell, Result-typed errors
    Session    *Session         // wraps SessionStorage
    Tools      map[string]Tool
    HookBus    *HookBus         // typed hooks + broadcast subscribers
    Resources  Resources        // skills + prompt templates
    Model      Model
    StreamOpts StreamOptions
    phase      Phase            // atomic enum
}

func (h *AgentHarness) Prompt(ctx context.Context, text string, opts ...PromptOption) (*AssistantMessage, error)
func (h *AgentHarness) Steer(text string) error
func (h *AgentHarness) FollowUp(text string) error
func (h *AgentHarness) Compact(customInstructions string) (*CompactResult, error)
func (h *AgentHarness) NavigateTree(targetID string, opts ...TreeOption) (*NavigateResult, error)
func (h *AgentHarness) On(event string, handler HookHandler) func() // returns unsubscribe
```

---

## What's elegant

- **Append-only event log = git log for sessions.** Forking, compacting, navigating between branches all become "append a new pointer entry."
- **`firstKeptEntryId` is a kept-tail anchor**, not a "delete from here" command. The summary entry plus the post-anchor entries form the new view; the pre-anchor history is preserved on disk.
- **Iterative compaction** with the previous summary as input produces stable, growing summaries instead of summary drift.
- **Phase machine** prevents concurrent ops without complex locking.
- **`pendingSessionWrites`** lets the model see consistent state mid-turn while deferring durability writes until safe boundaries.

## TS-specific (skip in Go)

- Module-augmentation tricks for `CustomAgentMessages` → use Go interfaces or sealed unions.
- `AggregateError` for combining errors during abort → `errors.Join`.
- Explicit getter/setter for `tools` / `messages` to copy on assign → Go can use methods that take slices and copy explicitly.