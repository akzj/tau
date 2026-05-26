# 08 — Migration Notes (pi → tau)

> **Scope**: Key design divergences between pi (TypeScript agent harness) and tau (Go agent kernel).
> **Reads with**: `04-design-principles.md` (every divergence maps to a principle).
> **Purpose**: For readers familiar with pi's architecture. Not a migration guide — a design philosophy map.

---

## §1 — Structural Divergences

| Pi (TypeScript) | Tau (Go) | Why |
|-----------------|----------|-----|
| `Agent` + `AgentHarness` — two classes, same package, same abstraction layer | Single `Loop` interface | R2: one concept, one canonical type. D1: pi's double-abstraction was a confirmed anti-pattern. |
| `agent.state` — flat mutable fields (messages, tools, systemPrompt) | `*Session` — first-class state container | Session is the isolation boundary (C1/R4). All mutable state lives here. No global state. |
| `agent.state.messages` — directly mutable array | `Transcript` — method-gated (Append/Slice/Subscribe) | C6: no internal loop mirror. Single source of truth. |
| `apiProviderRegistry` — package-level global `Map` | `ProviderRegistry` — field on `*Session` | R4: no package-level mutable state. Two sessions share nothing. |
| 4 entry points: `prompt`, `continue`, `steer`, `followUp` | 2 entry points: `Prompt`, `Continue` (internal) | Insufficient evidence for steer/followUp (A2 #4). Deferred to Phase 5+. |
| `AssistantMessageEvent` serves both raw-provider and product-layer | `AgentEvent` (semantic) + `ProviderEvent` (raw, internal) | Double-layer stream (A2 #2+3): shrink product boundary, hide raw tokens. |

---

## §2 — Provider Divergences

| Pi | Tau | Why |
|----|-----|-----|
| 30 vendors → one global registry, keyed by API kind | 9 wire protocols → `ProviderRegistry` on Session, `RegisterVendor` user-facing | Wire/vendor double-layer (Phase 3 audit discovery). Vendor = business entity; wire = HTTP contract. |
| `compat` field: `any` with type-narrowing by `api` | `WireCompat` sealed interface (3 concrete types) | R1: no `any` in public API. Compile-time sealed, runtime invariant check at registration. |
| `OpenRouterRouting` embedded in `OpenAICompletionsCompat` | `VendorTypedRouting` — separate sealed field on `VendorConfig` | Decouple vendor routing from wire compat (A2 Q4). |
| `getApiKey` — implicit per-call pattern | `OAuthSource` — explicit in `VendorConfig`, per-call contract | Make the token-refresh expectation visible in the type system. |
| `streamSimple`/`completeSimple` — package-level functions | `Provider.Complete` — interface method | R4: no package-level functions with side effects. |

---

## §3 — Tool & Hook Divergences

| Pi | Tau | Why |
|----|-----|-----|
| `tool.parameters as any` — generic erasure at boundary | `ToolSchema` interface — explicit Marshal/Validate contract | R1: cross-boundary type erasure forbidden. |
| `emitHook` (last-wins) + `emitBeforeProviderRequest` (chain) — same API, hidden semantics | `Chain[T]` + `LastWins[T]` — two distinct generic interfaces | C5: different semantics → different method sets. Compiler catches misuse. |
| `prepareArguments` shim — always-on pre-processing | `PrepareArgs` — optional field on `Tool` | Kept as compatibility valve for older models, but not mandatory. |
| `agent.beforeToolCall` / `agent.afterToolCall` — hook registration on Agent | `HookSet.BeforeToolCall` / `AfterToolCall` — fields on Session | Hooks are Session-scoped, not loop-scoped. |

---

## §4 — Schema Divergences

| Pi | Tau | Why |
|----|-----|-----|
| TypeBox: TypeScript type → JSON Schema (reflection-based) | Schema-first codegen: JSON Schema → Go struct | Go reflection can't express discriminated unions (`oneOf`). Codegen can. PoC-verified. |
| `models.generated.ts` — 16K LOC generated table in package | Model registry → product layer, not core | R5: core knows no product models. Provider implementations carry their own model lists. |
| Lazy loading via `import.meta.url` + dynamic import | Explicit `RegisterVendor` call per session | R4: no implicit init-side-effect registration. |

---

## §5 — Compaction Divergences

| Pi | Tau | Why |
|----|-----|-----|
| Full compaction engine in core (`compaction.ts`): cut-point algorithm, prompt template, file-op tracking | `BeforeCompaction` hook only — trigger point, not engine | G4 路径A: minimum viable. Core triggers, product implements. |
| `session:compact` event + `registerProvider` runtime extension | `HookSet.BeforeCompaction LastWins[CompactionRequest]` | Same pattern (core-triggered, product-replaced) but with type-safe generic interface. |

---

## §6 — Philosophy Shifts

| Pi Practice | Tau Principle | Impact |
|-------------|--------------|--------|
| `AGENTS.md` — rules for the agent, not enforced | R3 — every red line lint-enforced in CI | Tau prevents bugs at compile time that pi documents as conventions. |
| `biome.json:13` — explicitly disables `noExplicitAny` | R1 — custom vet analyzer bans `any` in public API | Tau's type system is a contract, not a suggestion. |
| "No backward compat unless asked" — social convention | R6 — philosophy freshness enforced by CI | Tau makes its own design philosophy first-class and machine-checked. |
| Multi-session safety via file-per-session convention | R4 — package-level mutable state structurally impossible | Tau eliminates the failure mode; pi relies on discipline. |

---

## §7 — Still Pi (Worthy of Respect)

Not everything in pi was wrong. These patterns survive in tau:

- **Faux provider** (B2): test the real loop with deterministic provider — tau keeps this.
- **Errors as values**: pi's `Result<T, E>` → tau's `(T, error)` is natural Go.
- **Streaming-only provider contract**: pi's `streamSimple` wraps `stream` → tau's `Stream` is the only path.
- **Append-only transcript**: pi's immutable event log → tau's method-gated `Transcript`.
- **Hook chain-of-responsibility**: pi's hook bus inspired tau's `Chain[T]` + `LastWins[T]`.

---

## §8 — Connection Points

| This file references... | Why |
|--------------------------|-----|
| `04-design-principles.md` | Every divergence maps to R1–R6 or P1–P2. |
| `01-core-design.md` | The abstractions that replace pi's Agent/AgentHarness. |
| `02-provider.md` | Wire/vendor double-layer replaces pi's flat `apiProviderRegistry`. |
| `06-compaction.md` | G4 路径A replaces pi's full compaction engine. |

---

*08-migration-notes.md — from pi's lessons, tau's laws.*
