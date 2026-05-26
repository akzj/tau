# 05 — Access Points

> **Scope**: Every surface where product-layer code touches the core — the complete API contract.
> **Basis**: Phase 2 v1.2 §5, A2 11 access points, webui multi-session requirement.
> **Reads with**: `01-core-design.md` (the abstractions being accessed), `02-provider.md` (provider registration).

---

## §1 — Tool Protocol

| Access Point | Signature | Driven by |
|-------------|-----------|-----------|
| Register tool | `sess.Tools.Register(Tool) error` | Coding-agent registers 7 built-in tools (read/bash/edit/write/grep/find/ls). |
| Set active tools | `sess.Tools.SetActive([]string)` | Dynamic tool enable/disable per conversation context. |
| Pre-execute hook | `sess.Hooks.BeforeToolCall.Add(handler)` | Permission gate, audit log, tool-use policy. |
| Post-execute hook | `sess.Hooks.AfterToolCall.Add(handler)` | Result rewriting, safety filtering, file-op extraction for compaction. |

**Caveat** (A1 §3): `ToolCallUpdate` events may reorder in parallel batches. Product layers needing strict partial→final ordering should self-sort by `CallID + seq`.

---

## §2 — Skills

| Access Point | Signature | Driven by |
|-------------|-----------|-----------|
| System prompt injection | `SessionOptions.SystemPrompt SystemPromptFn` | Coding-agent loads `SKILL.md` files via closure, formats, returns prompt. |

**Single entry point** — core needs no more. Skills are a product-layer concept (G3). The `SystemPromptFn` closure is the injection boundary.

---

## §3 — Event Stream (External Consumers)

| Access Point | Signature | Driven by |
|-------------|-----------|-----------|
| Subscribe to agent events | `run.Events <-chan AgentEvent` | TUI real-time rendering, WebSocket push to webui. |
| Subscribe to transcript | `sess.Transcript.Subscribe(cursor) (<-chan Message, func())` | Multi-client webui watching the same session; TUI restart resuming from cursor. |
| Wait for completion | `run.Done() (RunResult, error)` | Coding-agent in batch mode blocks on result. |

**Double-layer separation**: `run.Events` is semantic (MessageStart, ToolCallEnd, TurnEnd). Raw provider tokens are consumed internally by Loop. Product code never sees `ProviderEvent` (A2 #3).

---

## §4 — Multi-Session Concurrency

| Access Point | Signature | Driven by |
|-------------|-----------|-----------|
| Create session | `core.NewSession(ctx, opts) (*Session, error)` | Webui: one session per user, fully isolated (C1/R4). |
| Cancel session | `sess.Cancel()` | User closes webui tab → ctx done → all in-flight tools/providers cooperatively exit. |

**Guarantee**: two sessions share nothing. No locks, no "switch session", no inter-process coordination. R4 is structurally enforced — package-level mutable state does not exist in `core/`.

---

## §5 — Provider Registration

| Access Point | Signature | Driven by |
|-------------|-----------|-----------|
| Register vendor | `sess.Providers.RegisterVendor(name, cfg VendorConfig) error` | Coding-agent extension registers OpenAI, Anthropic, OpenRouter, etc. |
| Get provider | `sess.Providers.Get(name) (Provider, bool)` | Loop internals; product code typically doesn't call this directly. |

`RegisterVendor` is the **only** provider registration API. No `init()` auto-registration, no blank-import magic (R4), no global registry. Each session has its own `ProviderRegistry`.

---

## §6 — Single-Shot LLM (Bypassing Loop)

| Access Point | Signature | Driven by |
|-------------|-----------|-----------|
| One-shot complete | `sess.Providers.Get(name).Complete(ctx, req) (CompleteResponse, error)` | Compaction summarization, branch summarization — any non-interactive LLM call. |

`Complete` bypasses the Loop entirely. The caller constructs its own messages and receives a single response. This is the path for compaction handlers, branch summarizers, and any product-layer offline LLM use.

---

## §7 — Compaction

| Access Point | Signature | Driven by |
|-------------|-----------|-----------|
| Before compaction hook | `sess.Hooks.BeforeCompaction.Set(handler)` | Product layer provides the summarize implementation; core triggers at token threshold. |

See `06-compaction.md` for the full interface and rationale (G4 路径A).

---

## §8 — Minimal Demo

```go
func MinimalAgent(ctx context.Context, openaiKey string) error {
    sess, _ := core.NewSession(ctx, core.SessionOptions{
        SystemPrompt: func(s *core.Session) (string, error) {
            return "You are a helper. Use tools when needed.", nil
        },
    })
    defer sess.Cancel()

    sess.Providers.RegisterVendor("openai", openai.NewVendorConfig(openaiKey))
    sess.Tools.Register(core.Tool{
        Name: "echo", Description: "Echo back the input.",
        Schema: echoschema.Schema, // codegen-generated
        Execute: func(ctx context.Context, callID string, params any,
            onUpdate func(core.PartialResult)) (core.ToolResult, error) {
            args := params.(EchoArgs)
            return core.ToolResult{Content: []core.Content{{Text: args.Msg}}}, nil
        },
    })
    sess.Tools.SetActive([]string{"echo"})

    loop := core.NewLoop()
    run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "echo hello"})
    for ev := range run.Events { _ = ev }
    return run.Done()
}
```

≤30 lines, covers §1–§7. Writable → access points are closed-form.

---

## §9 — Connection Points

| This file → | Why |
|-------------|-----|
| `01-core-design.md` | Every access point surfaces on Loop, Session, Transcript, Tool, Hook. |
| `02-provider.md` | `RegisterVendor`, `Complete` — the provider-side access points. |
| `03-tool-schema.md` | `Tool.Schema` — codegen output gets registered here. |
| `06-compaction.md` | `BeforeCompaction` — the compaction hook access point. |

---

*05-access-points.md — the API contract between core and product.*
