# 07 — Go Module Structure

> **Scope**: The Go module layout for tau core, provider implementations, and generated code.
> **Basis**: `04-design-principles.md` (R4 no global state, R5 core knows no product).
> **Reads with**: `01-core-design.md` (what lives in `core/`), `02-provider.md` (what lives in `providers/`).

---

## §1 — Module Root

```
tau/
├── go.mod                  # module github.com/example/tau
├── core/                   # agent runtime kernel — the only required import
├── providers/              # wire-protocol implementations (one per subdirectory)
├── toolspec/               # tool schema definitions + codegen output
├── demo/                   # minimal working example (CI-compiled, not shipped)
└── docs/v2/                # this documentation set
```

**Single module**. No monorepo tricks. `core/` has zero external LLM dependencies.

---

## §2 — `core/` Package

```
core/
├── loop.go                 # Loop interface + Run struct
├── session.go              # Session struct + NewSession + SessionOptions
├── transcript.go           # Transcript struct (method-gated, hidden internals)
├── event.go                # AgentEvent sealed interface + 9 concrete event types
├── tool.go                 # Tool struct + ToolExecuteFn + ToolRegistry
├── provider.go             # Provider interface + StreamRequest + ProviderEvent + WireCompat
├── hook.go                 # Chain[T] + LastWins[T] + HookSet
├── errors.go               # ErrorCode + typed errors
```

**Rules**:
- No file >300 lines. Split by concern.
- No `init()` functions in `core/` (R4: no implicit global state).
- `core/` imports only the standard library + `toolspec/` for `ToolSchema`.
- `core/` NEVER imports `providers/` (providers implement `core.Provider`, not the reverse).

---

## §3 — `providers/` Package

```
providers/
├── openai-completions/     # implements core.Provider for WireOpenAICompletions
│   ├── provider.go         #   Stream + Complete
│   └── compat.go           #   OpenAICompletionsCompat ↔ HTTP mapping
├── anthropic-messages/     # implements core.Provider for WireAnthropicMessages
├── openai-responses/       # implements core.Provider for WireOpenAIResponses
├── google-generative-ai/
├── google-vertex/
├── bedrock-converse/
├── mistral-conversations/
├── azure-openai-responses/
└── openai-codex-responses/
```

- **One subdirectory per `WireAPI` constant**. Directory name = wire identifier (kebab-case).
- Each subdirectory exports: a constructor `NewVendorConfig(...) VendorConfig`, and internal `Provider` implementation (unexported).
- `providers/` imports `core/` (to implement `Provider` interface) — the dependency direction is correct.
- **Build tags** can exclude provider subdirectories to reduce binary size (e.g., `//go:build !noaws` for Bedrock).
- **Vendor-specific routing** (OpenRouter, Vercel Gateway, etc.) lives alongside the wire implementation, not in `core/`.

---

## §4 — `toolspec/` Package

```
toolspec/
├── codegen/                # go-jsonschema invocation + custom templates (Phase 5+)
├── echo.schema.json        # echo tool schema (source of truth)
├── echo_gen.go             # generated: typed struct + Marshal/Validate (DO NOT EDIT)
├── file-ops.schema.json    # file operation tool schema
├── file-ops_gen.go
└── ...                     # one .schema.json + _gen.go pair per tool
```

- **Schema is source**: `*.schema.json` files are hand-authored. CI runs `go generate` and diffs.
- **Generated code committed**: `*_gen.go` files are in version control — consumers don't need `go generate` to build.
- **Only `toolspec/` imports the codegen library**. `core/` only sees the `ToolSchema` interface.

---

## §5 — What Goes Where

| Concept | Location | Rationale |
|---------|----------|-----------|
| Loop, Session, Transcript, Tool, Hook, AgentEvent | `core/` | Kernel abstractions — the tau runtime itself. |
| Provider interface, StreamRequest, WireCompat, VendorConfig | `core/` | Provider contract — core defines, providers implement. |
| OpenAI Completions HTTP wiring | `providers/openai-completions/` | Wire-specific implementation detail. |
| Tool schema definitions | `toolspec/` | Separate from core (core only sees interface). |
| Built-in tools (bash, read, write, grep) | **Product repo** | Not in tau core or providers. R5 forbids product names in `core/`. |
| Skills loading, prompt assembly | **Product repo** | Skills are product-layer (G3). |
| Harness, steering, follow-up | **Product repo** | Deferred to Phase 5+. |
| TUI, webui, coding-agent | **Product repo** | Entirely separate Go module. |

---

## §6 — Build Order (V0.1 Milestone)

1. **`core/`**: all type definitions, zero implementations. Compiles standalone.
2. **`toolspec/`**: one `.schema.json` → `go generate` → compiles with `core/`.
3. **`providers/openai-completions/`**: first wire implementation. Compiles with `core/`.
4. **`demo/`**: minimal echo agent (§5 access-points demo). Compiles with `core/` + `providers/` + `toolspec/`.

After V0.1: iterate by adding providers and tools. The dependency graph is a DAG from the start — `core/` at the bottom, everything else builds on it.

---

## §7 — Connection Points

| This file → | Why |
|-------------|-----|
| `01-core-design.md` | `core/` package layout maps 1:1 to abstractions. |
| `02-provider.md` | `providers/` subdirectories per `WireAPI`. |
| `03-tool-schema.md` | `toolspec/` = schema files + generated code. |
| `05-access-points.md` | `demo/` implements the minimal demo from §8. |

---

*07-go-structure.md — core at the bottom, providers on top, product elsewhere.*
