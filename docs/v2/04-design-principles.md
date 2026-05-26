# 04 — Design Principles

> **Role**: Foundation document. Every other `docs/v2/` file assumes these principles.
> **Sources**: Phase 2 v1.2 §1+§3, Handoff C1-C10, LTM decisions 002/005/011.
> **Scope**: Why tau is designed this way — not what tau does (see 01–03, 05–07).

---

## §1 — Five Core Assertions

These are tau's architectural defaults. Every abstraction in 01-core-design.md derives from them.

| # | Assertion | What it means |
|---|-----------|---------------|
| 1 | **Types are truth** | Go type signatures = runtime contract. `interface{}` / `any` in public API is a compile error. No unguarded type assertions. R1 enforces this with lint + vet. |
| 2 | **Streams are functions, not objects** | Provider is a function producing an event channel, not a stateful client with lifecycle. C10 defines this; testing (Faux) and cancellation build on it. |
| 3 | **Session is the boundary** | All mutable state lives on `*Session`. No package-level mutable maps, no global registries. Two sessions share nothing. R4 enforces this with vet + grep. |
| 4 | **Core knows no product** | `core/` packages contain zero references to "Claude", "GPT", "Anthropic", "bash", "git", "TUI", "webui". Provider-specific logic lives behind the `Provider` interface. R5 enforces this with CI grep. |
| 5 | **Every red line has a lint** | A rule without automated enforcement is not a rule. CI is the commitment. R3 is the meta-rule that enforces this recursively — including on itself. |

**Why these five**: They are the distillation of pi's real-world failures (D1–D13 in Phase 1). Every one maps to a class of bug that pi hit in production and tau prevents at compile time.

---

## §2 — R1–R6 Permanent Rules

The six rules that govern all tau design decisions. No exception without Owner approval.

| Rule | Statement | Enforcement |
|------|-----------|-------------|
| **R1** | Type = runtime reality | `golangci-lint` `forcetypeassert` + custom vet analyzer banning `interface{}` / `any` in public API signatures. Cross-abstraction-boundary type erasure forbidden. |
| **R2** | Single canonical abstraction | One concept → one public type. Architecture review: new public type must answer "does this concept already have an abstraction?" CI grep gate detects same-concept keyword duplication. |
| **R3** | Every red line lint-enforced | Meta-rule: every rule in this document must map to a concrete lint/vet/grep in CI. `ci/check_redline_coverage.sh` verifies doc ↔ lint config sync. |
| **R4** | Registry scoped to Session | Custom vet analyzer bans package-level `map`, `sync.Map`, or mutable global in `core/`. All registries created via `New*Registry()` factory. |
| **R5** | Abstraction layer knows no product | CI grep for product names (`Claude`, `GPT`, `Anthropic`, `coding-agent`, `TUI`, `webui`) in `core/` → fail. Allowlist for spec-documentation quotes only. |
| **R6** | Design philosophy is first-class doc | This file must exist. Every PR changing core public API must update it or justify "no philosophy change". CI checks mtime freshness. |

**Known compromise**: R2 cannot be 100% automated — "same concept, two type names" is natural-language. The grep heuristic catches ~80%; architecture review covers the gap.

---

## §3 — P1 + P2 Supplementary Principles

Discovered during Phase 2 design evolution (LTM `team/decisions/005`).

### P1 — "Review pain < User pain"

When a design trade-off is "developer faces annoyance" vs "user faces a bug", **always solve the user pain first**.

- **Application**: Compile-time checks beat runtime assertions. Two distinct `interface` types (Chain vs LastWins) beat one interface with a mode flag. Extra types beat silent failures.
- **Relationship to R3**: R3 is P1 applied to the "red line" domain — converting "developer might forget" into "toolchain won't allow."

### P2 — "Panic-fast = MustCompile pattern"

Errors that occur at program-init and are **configuration bugs** (not user-input errors) should **panic**, not return `error`.

- **When**: `init()` phase, registration of hooks/providers, duplicate config, API/Compat mismatch.
- **When not**: Request-handling phase, user-input validation, recoverable runtime conditions.
- **Go precedent**: `regexp.MustCompile`, `sync.Once.Do`, package `init` panics.
- **tau usage**: Duplicate `HookSet.LastWins.Set()` → panic. `RegisterVendor` with API/Compat mismatch → panic.

---

## §4 — Design Anti-Patterns (from pi)

Concrete failures that the principles above prevent. Source: Phase 1 pi critique + Phase 3 audit.

| Anti-pattern | Pi behavior | Tau prevention |
|-------------|------------|----------------|
| **Double abstraction** | `Agent` + `AgentHarness` in same package, same abstraction layer, overlapping responsibilities (D1). | Single `Loop` interface. One concept, one type (R2). |
| **Global mutable registry** | `apiProviderRegistry` is package-level `Map<string, Provider>`, mutated at runtime by extensions (D4). | `ProviderRegistry` is a field on `*Session` (R4). No global state. |
| **Hook API with hidden semantics** | `emitHook` is last-wins; `emitBeforeProviderRequest` is chain. Same API surface, different behavior — caller can't tell statically (D5). | `Chain[T]` and `LastWins[T]` are two distinct generic interfaces with different method sets (C5). Compiler catches misuse. |
| **Implicit transcript mirror** | Loop maintains internal message list, invisible to subscribers. `transformContext` reads from hidden mirror, not from the public transcript (D7). | `Transcript` is the single source of truth (C6). Loop reads from it via `Slice`. No hidden mirrors. |
| **Product knowledge in core** | `anthropic.ts` hardcodes 17 Claude Code tool names; `isOAuthToken` forces tool renaming (D13). | `TransformToolName` is a caller-injected function on `StreamRequest`. Core knows zero tool names (C7). |
| **Lazy init via blank-import** | TS `import.meta.url` munging; Go sketch suggests `init()` auto-registration. | Providers registered **explicitly** via `sess.Providers.RegisterVendor(...)`. No magic init (R4). |

---

## §5 — Connection Points

| This file references... | Why |
|--------------------------|-----|
| `01-core-design.md` | Abstractions (Loop, Session, Transcript, Hook) that embody R1–R6 and P1–P2. |
| `02-provider.md` | Provider registry (R4 scoping), wire/vendor double-layer (R5 product无知), Compat sealed (R1 type-safety). |
| `03-tool-schema.md` | B9 codegen decision — schema-first ensures R1 (单一来源) and R3 (CI diff catches drift). |
| `06-compaction.md` | G4 路径A: `BeforeCompaction LastWins[CompactionRequest]` — P2 panic-fast on duplicate registration. |
| `07-go-structure.md` | Package layout enforces R5 (core/ has zero product strings in CI grep). |
| `08-migration-notes.md` | Every divergence-from-pi listed there maps to one of these principles. |

---

*04-design-principles.md — the tau constitution. Violate these and you violate tau.*
