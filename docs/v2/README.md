# docs/v2 — Tau Design Documentation

> **Status**: Phase 4 PRODUCE — formal specification, not draft.
> **Tau**: A Go agent runtime kernel + provider abstraction. Minimum controllable runtime for LLM agents.

---

## Reading Order

| # | File | Reads | What it covers |
|---|------|-------|----------------|
| 1 | `04-design-principles.md` | ~5 min | Why tau is designed this way: 5 assertions, R1–R6, P1+P2, pi anti-patterns. |
| 2 | `01-core-design.md` | ~10 min | The six kernel abstractions: Loop, Session, Transcript, AgentEvent, Tool, Hook. |
| 3 | `02-provider.md` | ~12 min | Provider interface, wire/vendor double-layer, Compat sealed system. |
| 4 | `03-tool-schema.md` | ~8 min | ToolSchema, B9 codegen decision, `toolspec/` workflow. |
| 5 | `06-compaction.md` | ~6 min | G4 路径A: `BeforeCompaction` hook. What core does and doesn't do. |
| 6 | `05-access-points.md` | ~8 min | Every product-layer API surface. Minimal demo. |
| 7 | `07-go-structure.md` | ~5 min | Module layout, package boundaries, build order. |
| 8 | `08-migration-notes.md` | ~7 min | pi → tau design divergence map. For readers familiar with pi. |

**Total**: ~60 minutes. Each file is self-contained but builds on earlier files.

## Quick Start

```bash
go build ./...         # core/ + toolspec/ compile standalone
(cd demo && go build)  # minimal echo agent — verifies access points
```

## Key Decisions (already ruled)

- **G4**: `BeforeCompaction LastWins[CompactionRequest]` (路径A) — see `06-compaction.md`
- **B9**: Schema-first codegen (`atombender/go-jsonschema`) — see `03-tool-schema.md`
- **Wire-protocol**: Form B double-layer (9 wire impls, `RegisterVendor` user surface) — see `02-provider.md`
- **Compat**: Sealed interface (3 concrete types, no `any`) — see `02-provider.md` §4

---

*docs/v2 — the tau specification. Phase 4 PRODUCE.*
