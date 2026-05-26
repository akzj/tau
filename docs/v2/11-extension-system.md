# 11 — Extension System

> **Scope**: User-installable extension system for tau coding-agent — runtime, events, API, hot-reload.
> **Basis**: `docs/06-self-extensibility.md` (pi analysis), `01-core-design.md`, `05-access-points.md`.
> **Status**: Design phase. Product-layer only — zero core changes.

---

## §1 — Go Runtime Options

pi uses jiti (`import` TS at runtime + virtualModules for in-binary resolution). Go cannot do this.
Four options evaluated:

| Option | Mechanism | Pros | Cons | Verdict |
|--------|-----------|------|------|---------|
| **goja** | JS VM (ES5.1+, partial ES6) | Familiar for pi migrators; sandboxable; in-process; no external runtime | Not native Go; host-object marshaling overhead | ✅ **v1 choice** |
| Starlark | Python-flavored, deterministic | Best sandbox (no I/O by default); Go-native VM | Unfamiliar for JS devs; limited stdlib | Alternative |
| Subprocess | JSON-RPC over stdio | Language-agnostic; clean isolation; Go extensions possible | Per-extension startup cost; serialization overhead every callback | v2 path |
| Wasm (wazero) | WebAssembly sandbox | Real sandbox; deterministic; future-proof | Upfront work; smaller ecosystem; tooling gap | v2+ |
| Go `plugin` | `plugin.Open()` | Go-native | UNIX-only; exact Go version match; no reload; no sandbox | ❌ Avoid |

**Decision: goja for v1.** Extensions are JS files dropped into `.tau/extensions/`. The goja VM
exposes `tau.*` host object (ExtensionAPI). Subprocess architecture planned for v2 when
Go-native extensions are demanded.

---

## §2 — Event Mapping: pi 90+ Events → tau

pi's extension system exposes 90+ event types across 7 categories. Tau maps these to its
existing `AgentEvent` sealed interface + `HookSet` + `ProviderEvent` — **no new core event types needed**.

### Category 1: Agent Lifecycle → Loop Events (mapped via event stream)

| pi Event | tau Mechanism |
|----------|--------------|
| `agent:start`, `agent:end` | `run.Events` — no direct equivalent; extension runtime synthesizes from first `TurnStart` + `run.Done()` |
| `turn:start`, `turn:end` | `run.Events` → filter `TurnStart` / `TurnEnd` |

### Category 2: Message Lifecycle → AgentEvent stream

| pi Event | tau AgentEvent |
|----------|---------------|
| `message:start` | `MessageStart` |
| `message:update` | `MessageDelta` (content merged, not raw tokens) |
| `message:end` | `MessageEnd` |

### Category 3: Tool Execution → AgentEvent stream + HookSet

| pi Event | tau Mechanism |
|----------|--------------|
| `tool:start` | `ToolCallStart` (event stream) + `BeforeToolCall` chain |
| `tool:update` | `ToolCallUpdate` (event stream) |
| `tool:end` | `ToolCallEnd` (event stream) + `AfterToolCall` chain |

### Category 4: Provider → HookSet + StreamRequest callbacks

| pi Event | tau Mechanism |
|----------|--------------|
| `beforeProviderRequest` | `HookSet.BeforeProviderRequest` (LastWins, can mutate request) |
| `afterProviderResponse` | `StreamRequest.OnResponse` callback (per-request, not global) |

### Category 5: Session Lifecycle → Extension Runtime (product layer)

| pi Event | tau Implementation |
|----------|-------------------|
| `session:start` | Extension runtime fires on `NewCodingSession` |
| `session:shutdown` | Extension runtime fires on `sess.Cancel()` |
| `session:beforeCompact` / `session:compact` | `HookSet.BeforeCompaction` (LastWins) |
| `session:beforeFork` / `session:tree` | Product layer only — tau Transcript is flat; branching built on top |
| `session:beforeSwitch` | Product layer only — multi-session management |

### Category 6: User Interaction → Product Layer

| pi Event | tau Implementation |
|----------|-------------------|
| `input` (slash commands) | Extension `registerCommand` — CLI dispatches to registered handlers |
| `userBash` | `BeforeToolCall` hook filtered by `ToolName == "bash"` |
| `modelSelect` / `thinkingLevelSelect` | Product layer — not core events |

### Category 7: Resources → Product Layer

| pi Event | tau Implementation |
|----------|-------------------|
| `resources:discover` | Product layer — extension calls `registerTool` / `registerProvider` during init |
| `context` (dynamic injection) | `HookSet.TransformContext` chain |

### Coverage: ~22 of pi's 90+ events are "core" events (agent/turn/message/tool/provider).
The remaining ~70 are product-layer UX events (TUI widgets, keybindings, autocomplete, theming)
— out of scope until tau's TUI phase.

---

## §3 — Hook vs Extension: Complementary, Not Redundant

```
┌──────────────────────────────────────────────┐
│  Extension System (pkg/coding/extensions/)    │
│  • JS runtime (goja)                          │
│  • Hot-reload via fsnotify                    │
│  • Stale-context invalidation                 │
│  • Pre-bind queuing                           │
│                                               │
│  ┌──────────────────────────────────────┐     │
│  │  tau HookSet (core/)                  │     │
│  │  BeforeToolCall, AfterToolCall,       │     │
│  │  TransformContext, BeforeCompaction,  │     │
│  │  BeforeProviderRequest                │     │
│  └──────────────────────────────────────┘     │
└──────────────────────────────────────────────┘
```

**Hooks are the interception points.** Extensions are the **distribution mechanism** that lets
users drop files into `.tau/extensions/` and register handlers without recompiling.

Relationship:
- **Hooks without extensions**: Works. You can write Go code that calls `sess.Hooks.BeforeToolCall.Add(...)`.
- **Extensions without hooks**: Impossible. Extensions need something to hook into.
- **Extensions WITH hooks**: The tau model. Extensions register handlers on hooks via goja host objects.

**Extensions do NOT replace hooks.** They are a consumer of hooks.

---

## §4 — Hot-Reload Strategy

Go cannot jiti-compile files at runtime. Three viable strategies:

### Strategy A: File Watcher + VM Recreate (v1)

```
fsnotify on .tau/extensions/  →  kill goja VM  →  create new VM  →  re-run all scripts
```

- Extensions are expected to be **stateless** (state lives in Session, not in extension VM).
- Reload latency: ~50ms for 10 extensions (goja VM creation is fast).
- No state migration needed — re-registration is idempotent.
- Limitation: in-flight tool calls from old VM are lost on reload (acceptable — they were registered on old hooks, which are replaced).

### Strategy B: No Hot-Reload (simplest)

Reload on `tau --reload-extensions` or restart. No watcher. Acceptable if extension development
is infrequent.

### Strategy C: Per-Extension VM Pool (v2)

Each extension gets its own goja VM. Only changed extensions reload. Unchanged VMs persist.
Higher complexity, smoother UX.

**Decision: Strategy A for v1** — `fsnotify` watcher + full VM recreate. Simple, predictable.
Degrade to Strategy B if watcher proves unreliable.

---

## §5 — Extension File Format

Extensions are JS files (ES5.1+ subset supported by goja):

```javascript
// .tau/extensions/redraws.js
export default function(tau) {
    tau.registerCommand("tui-stats", {
        description: "Show TUI statistics",
        handler: function(args, ctx) {
            ctx.sendMessage("TUI stats: ...");
        },
    });

    tau.on("tool:start", function(event, ctx) {
        if (event.toolName === "bash") {
            ctx.sendMessage("Running: " + event.args.command);
        }
    });
}
```

### Frontmatter Convention (optional)

```javascript
/// name: "tui-stats"
/// description: "TUI statistics slash command"
/// version: "1.0.0"

export default function(tau) { ... }
```

Parsed by extension loader for display in `tau extensions list`.

---

## §6 — Go API Draft (Host Objects)

The goja VM exposes a `tau` global implementing `ExtensionAPI`:

```go
// pkg/coding/extensions/api.go

// ExtensionAPI is the host object exposed to goja scripts as global `tau`.
type ExtensionAPI struct {
    // --- Event Subscriptions ---
    // tau.on("turn:start", handler)
    // tau.on("message:start", handler)
    // tau.on("tool:start", handler)
    // tau.on("tool:end", handler)
    // tau.on("beforeProviderRequest", handler)
    // tau.on("session:start", handler)
    // tau.on("session:shutdown", handler)
    On func(eventName string, handler goja.Callable)

    // --- Registrations ---
    RegisterTool    func(def map[string]interface{}) // {name, description, schema, handler}
    RegisterCommand func(name string, def map[string]interface{}) // {description, handler}
    RegisterProvider func(name string, cfg map[string]interface{}) // {api, baseURL, apiKey, models}
}

// ExtensionContext is passed as second arg to event handlers and command handlers.
type ExtensionContext struct {
    SessionID string

    // Actions
    SendMessage    func(text string)
    SendUserMessage func(text string)
    GetActiveTools func() []string
    SetActiveTools func(names []string)

    // Session management (with stale-context invalidation)
    NewSession  func(opts map[string]interface{}) (ExtensionContext, error)
    Fork        func() (ExtensionContext, error)
    SwitchSession func(id string) (ExtensionContext, error)
    Reload      func() error
}
```

### Stale-Context Safety (pi pattern preserved)

After `ctx.NewSession()`, `ctx.Fork()`, `ctx.SwitchSession()`, or `ctx.Reload()`, the captured
`ctx` becomes **stale**. All method calls on a stale context return a descriptive error:

> "This extension context is stale after session replacement. Use the context returned by NewSession/Fork/SwitchSession."

Implementation: each context has an `active bool`. Session-replacement methods set `active=false`
on the old context before returning the new one.

### Pre-Bind Queueing (pi pattern preserved)

Registrations (`registerTool`, `registerProvider`) during extension load are queued and
flushed when the core session is ready. Action methods (`sendMessage`, `newSession`) called
before binding return an error: "Extension not yet bound to session."

---

## §7 — Routing Table: Extension Events → Core Mechanisms

| Extension Event | Core Mechanism | Category |
|----------------|---------------|----------|
| `turn:start` | `run.Events` → filter `TurnStart` | Event stream |
| `turn:end` | `run.Events` → filter `TurnEnd` | Event stream |
| `message:start` | `run.Events` → filter `MessageStart` | Event stream |
| `message:update` | `run.Events` → filter `MessageDelta` | Event stream |
| `message:end` | `run.Events` → filter `MessageEnd` | Event stream |
| `tool:start` | `run.Events` → filter `ToolCallStart` + `BeforeToolCall` chain | Event + Hook |
| `tool:update` | `run.Events` → filter `ToolCallUpdate` | Event stream |
| `tool:end` | `run.Events` → filter `ToolCallEnd` + `AfterToolCall` chain | Event + Hook |
| `beforeProviderRequest` | `HookSet.BeforeProviderRequest` (LastWins) | Hook |
| `afterProviderResponse` | `StreamRequest.OnResponse` callback | Hook (per-request) |
| `beforeCompaction` | `HookSet.BeforeCompaction` (LastWins) | Hook |
| `session:start` | Extension runtime fires on `NewCodingSession` | Product layer |
| `session:shutdown` | Extension runtime fires on `sess.Cancel()` | Product layer |
| `input` (slash commands) | Extension `registerCommand` → CLI dispatch | Product layer |
| `userBash` | `BeforeToolCall` hook, filter `ToolName=="bash"` | Hook |
| `modelSelect` | Product layer only | Not core |
| `resources:discover` | Product layer (registerTool/Provider during init) | Not core |

**No new core events needed.** All 17 extension trigger points map to existing mechanisms.
The remaining ~70 pi events are TUI/UX events — out of scope pending TUI phase.

---

## §8 — Package Layout

```
pkg/coding/extensions/
  api.go             ExtensionAPI host object (goja)
  context.go         ExtensionContext + stale-context invalidation
  loader.go          Discover .tau/extensions/*.js, create VM, run scripts
  watcher.go         fsnotify watcher for hot-reload (Strategy A)
  queue.go           Pre-bind registration queue + flush
  events.go          Event stream fan-out: run.Events → extension handlers
  commands.go        Slash-command registry + dispatch
  sandbox.go         goja VM sandbox config (disable network, limit CPU)
```

---

## §9 — Sandbox (v1)

Extensions run in goja VM with:
- **No network**: `fetch`, `XMLHttpRequest`, `WebSocket` not exposed
- **No filesystem**: `fs` not exposed (extensions interact with files via `tau.registerTool`)
- **CPU limit**: goja `Interrupt` after 5s per handler invocation
- **No `eval`**: goja doesn't support dynamic eval in ES5.1 strict mode

Extensions are **trusted user scripts** (pi model), not an app-store plugin model. The sandbox
is a safety net, not a security boundary against malicious code.

---

## §10 — What is NOT in v1

| Concern | Status | Rationale |
|---------|--------|-----------|
| Subprocess extensions (Go-native) | v2 | goja covers JS-first users; Go devs use HookSet directly |
| Wasm sandbox | v2+ | wazero integration requires more design |
| TUI widget/keybinding extensions | TUI phase | No TUI yet |
| Extension marketplace / registry | Never core | Distribution is a community concern |
| Extension signing / verification | v2 | Trust model is user-installed |
| Per-extension VM pool (hot-reload Strategy C) | v2 | Full VM recreate is sufficient for <20 extensions |

---

## §11 — Core Gaps

**None.** The extension system is a product-layer consumer of existing core abstractions:

- Event stream: `run.Events` (`<-chan AgentEvent`) + fan-out goroutine
- Hook interception: `HookSet.BeforeToolCall`, `AfterToolCall`, `TransformContext`, `BeforeCompaction`, `BeforeProviderRequest`
- Provider callbacks: `StreamRequest.OnResponse`
- Session lifecycle: `core.NewSession` + `sess.Cancel()`

**Zero core source file modifications required.**

The only "new" mechanism is the goja VM — which lives entirely in `pkg/coding/extensions/`.

---

*11-extension-system.md — goja runtime, 17 mapped events, Hook+Extension complementary, hot-reload via fsnotify, zero core changes.*