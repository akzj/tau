# 06 — Compaction

> **Scope**: The compaction hook (G4 路径A), its relationship to Transcript and Provider.Complete, and why there is no core compaction engine.
> **Basis**: LTM `team/decisions/011` (G4 最终裁决), `04-design-principles.md` (P1+P2).
> **Reads with**: `01-core-design.md` (HookSet, Transcript), `02-provider.md` (Complete).

---

## §1 — Decision

**G4 路径A** (LTM `team/decisions/011`): Add `BeforeCompaction LastWins[CompactionRequest]` to `HookSet`.

- Cost: ~50 lines + 1 new public type + 1 new hook field.
- Semantics: core triggers at token threshold → handler replaces the summarize implementation → core takes over the append to Transcript.

This is the **only** compaction mechanism in core. There is no compaction engine, no cut-point algorithm, no prompt template, no file-op tracking. Those are product-layer concerns.

---

## §2 — Interface

```go
package core

type CompactionRequest struct {
    Summary          string     // replacement summary (set by handler)
    FirstKeptEntryID Position   // entries before this are compacted away
    TokensBefore     int        // token count that triggered the threshold
}

// HookSet gains:
type HookSet struct {
    // ... existing fields ...
    BeforeCompaction LastWins[CompactionRequest]  // G4 路径A
}
```

**Registration**:
```go
sess.Hooks.BeforeCompaction.Set(func(ctx context.Context, req CompactionRequest) (*CompactionRequest, error) {
    // 1. Read messages [0, req.FirstKeptEntryID) from sess.Transcript
    // 2. Call sess.Providers.Get("cheap-model").Complete(ctx, summarizeReq)
    // 3. Set req.Summary = result
    // 4. Return &req
    return &req, nil
})
```

**Semantics**:
- **`LastWins`**: exactly one handler decides the summary. Multiple registrations → panic (P2).
- **Core triggers**: when the token count crosses a product-configured threshold, core calls `BeforeCompaction.Run`.
- **Handler does the work**: writes `req.Summary`, returns. Core then appends the summary to Transcript and truncates the compacted range.
- **Handler can delegate**: use any model (cheaper, local, deterministic algorithm), call any Provider on the Session. The hook is an injection point, not an engine.

---

## §3 — Why Only Path A

### Path B (rejected for now): Transcript.AppendCompactionSummary

Adding `AppendCompactionSummary(summary, firstKeptEntryID Position, tokensBefore int) error` to Transcript would give product code a direct "summarize → replace tail" API without going through a hook.

**Not selected** because:
1. Pi's actual compaction is core-triggered (`agent-session.ts:1665/1938`), not product-triggered.
2. Path B can be added later as a non-breaking addition. Path A + Path B coexist without conflict.
3. Product-triggered compaction has zero evidence of real-world need. Don't build what isn't validated.

### Path C (rejected): A + B simultaneously

Over-engineering. Two paths to the same outcome violates R2 (single canonical abstraction).

---

## §4 — What Core Does NOT Provide

These are **product-layer responsibilities**, deliberately excluded from core:

| Concern | Why excluded | Where it lives |
|---------|-------------|----------------|
| Token threshold config | Product decides when to compact. | `SessionOptions` or product-layer policy. |
| Cut-point algorithm | "Don't split a tool call from its result" is a product-layer heuristic. | The `BeforeCompaction` handler's summarization logic. |
| Prompt template | "Summarize these messages..." is a product concern. | Inside the handler's `Complete` call. |
| File-op tracking | `<read-files>` / `<modified-files>` blocks are product-specific. | Product layer accumulates these from `AfterToolCall` and feeds them into the summary prompt. |
| Previous summary as input | "Feed previous summary to keep summaries stable across compactions" — product-layer algorithm. | Handler reads previous summary from Transcript. |

Core provides the **trigger point** and the **Transcript append/truncate mechanism**. Everything else is product code — and the hook is the boundary.

---

## §5 — Evolution History

**Phase 2 v1.0**: G4 decided "砍" — no compaction hook at all. Rationale: (1) ~50 lines cost, (2) zero known alternative implementations, (3) redundant with `Provider.Complete + Transcript`.

**Phase 3 audit**: Self-doubt #7 pre-commit trigger hit — pi's `custom-compaction.ts:9-115` uses a core-triggered hook (`session:compact` event + `registerProvider` runtime extension point). The "zero alternative implementations" claim was false.

**Phase 4 (current)**: Decision withdrawn → three paths evaluated → 路径A selected. The hook exists but at minimum viable scope — trigger point only, no engine.

---

## §6 — Connection Points

| This file → | Why |
|-------------|-----|
| `01-core-design.md` | `HookSet` is the insertion point; `Transcript` is the storage. |
| `02-provider.md` | `Provider.Complete` is how the handler does the actual summarization. |
| `04-design-principles.md` | P2 (duplicate `Set` → panic), R2 (single path for compaction). |
| `05-access-points.md` | The `BeforeCompaction` hook is one access point. |

---

*06-compaction.md — core triggers, product decides, Transcript remembers.*
