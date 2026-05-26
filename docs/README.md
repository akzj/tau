# Pi Architecture — Extraction for Go Port

This directory contains a design extraction of the pi agent harness, intended as a guide for porting pi's architectural ideas to Go.

The goal is **not a line-by-line translation** of TypeScript to Go. The goal is to extract the *design* — the abstractions that make pi work — and recommend how to express them idiomatically in Go.

Pi is unusually disciplined: layered, hook-rich, opinionated about message shape, and built around an **append-only event log** rather than mutable state. The most reusable pieces live in three places: the **agent loop**, the **harness/session log**, and the **compaction algorithm**.

---

## Reading order

| # | File | What it covers |
|---|------|----------------|
| 1 | [01-agent-loop.md](01-agent-loop.md) | The core LLM + tool execution cycle. Termination model. Streaming contract. Agent ↔ Harness split. |
| 2 | [02-harness.md](02-harness.md) | Session tree, append-only JSONL storage, **compaction** (the most reusable piece), system prompt assembly, hook bus. |
| 3 | [03-tool-protocol.md](03-tool-protocol.md) | Tool definition shape, registration, execution model (sequential/parallel), error handling. |
| 4 | [04-provider-abstraction.md](04-provider-abstraction.md) | Unified message types, streaming protocol, provider plugin shape, the **wire-protocol-as-unit-of-pluggability** insight, model registry. |
| 5 | [05-multi-session-safety.md](05-multi-session-safety.md) | How pi achieves multi-session safety without filesystem locks. |
| 6 | [06-self-extensibility.md](06-self-extensibility.md) | The `.pi/extensions/` mechanism. Stale-context safety. Recommended Go alternatives (Starlark / goja / Wasm). |
| 7 | [07-testing-faux-provider.md](07-testing-faux-provider.md) | The faux provider pattern: testing the real loop/harness/compaction with deterministic outputs. |
| 8 | [08-design-principles.md](08-design-principles.md) | Cross-cutting principles, **top 5 borrowable ideas**, what NOT to replicate, surprises. |
| 9 | [09-go-module-structure.md](09-go-module-structure.md) | Recommended Go module layout. Build order. |

---

## Top-line discoveries

If you only read three things, read these:

1. **Append-only session tree with `LeafEntry` as a navigation event** is THE big idea. JSONL on disk, in-memory `byId` map, `setLeafId` is itself an entry. ~300 LOC in Go.

2. **Compaction is layered**: cut-point validation (never split a `toolCall` from its `toolResult`) + iterative summary update (previous summary fed into the next compaction's prompt) + file-op accumulation across compactions.

3. **The provider abstraction's unit of pluggability is the WIRE PROTOCOL, not the vendor.** One `openai-completions` plugin handles ~30 vendors via a `compat` field with 20+ flags.

---

## Methodology used to produce this extraction

- **Read in full**: `packages/agent/src/index.ts`, `agent.ts`, `agent-loop.ts`, `types.ts`; the entire `harness/` tree (`agent-harness.ts`, `types.ts`, `messages.ts`, `system-prompt.ts`, `skills.ts`, `prompt-templates.ts`, all of `compaction/`, all of `session/`).
- **Read in full**: `packages/ai/src/index.ts`, `types.ts`, `api-registry.ts`, `stream.ts`, `models.ts`, `utils/event-stream.ts`, `providers/faux.ts`, `providers/register-builtins.ts`. Spot-checked top of `openai-completions.ts` and `anthropic.ts`.
- **Spot-checked extension API**: `packages/coding-agent/src/core/extensions/index.ts` + first 200 LOC of `loader.ts` + a real extension (`.pi/extensions/redraws.ts`).
- **Read** `AGENTS.md`, `CONTRIBUTING.md`, `README.md` for design philosophy.

### What was NOT covered

- The `packages/coding-agent` application layer beyond extensions.
- The `packages/tui` package (different concern; out of scope).
- Per-vendor provider implementations (Anthropic, Google, Bedrock, Mistral) — extracted the abstraction from the registry + faux + completions reference. If you need wire-protocol differences, that's a separate dive.
- No tests were run.