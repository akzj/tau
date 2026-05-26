# 11 — Extension System

> **Scope**: User-installable extension system — runtime, events, API, hot-reload.
> **Basis**: `docs/06-self-extensibility.md` (pi analysis), `01-core-design.md`, `05-access-points.md`.
> **Status**: Design phase. Product-layer only — zero core changes.

---

## §1 — Go Runtime Options

pi uses jiti (import TS at runtime + virtualModules). Go cannot do this.

| Option | Mechanism | Pros | Cons | Verdict |
|--------|-----------|------|------|---------|
| **goja** | JS VM (ES5.1+, partial ES6) | Familiar for pi migrators; sandboxable; in-process | Not native Go; host-object marshaling | ✅ **v1** |
| Starlark | Python-flavored, deterministic | Best sandbox; Go-native VM | Unfamiliar for JS devs; limited stdlib | Alt |
| Subprocess | JSON-RPC over stdio | Language-agnostic; clean isolation | Startup cost per extension; serialization overhead | v2 |
| Wasm (wazero) | WebAssembly sandbox | Real sandbox; deterministic | Upfront work; tooling gap | v2+ |
| Go `plugin` | `plugin.Open()` | Go-native | UNIX-only; version-fragile; no reload; no sandbox | ❌ |

**Decision: goja for v1.** Extensions = JS files in `.tau/extensions/`. goja VM exposes `tau.*` host object.
Subprocess architecture planned for v2 when Go-native extensions are demanded.

---

## §2 — Event Mapping: pi 90+ Events → tau

pi has 90+ events across 7 categories. tau maps to existing `AgentEvent` + `HookSet` + `ProviderEvent`.
**No new core event types needed.**

### Agent & Turn Lifecycle

| pi Event | tau Mechanism |
|----------|--------------|
| `agent:start`, `agent:end` | Synthesized from first `TurnStart` + `run.Done()` |
| `turn:start`, `turn:end` | `run.Events` → filter `TurnStart` / `TurnEnd` |

### Message Lifecycle → AgentEvent stream

| pi Event | tau AgentEvent |
|----------|---------------|
| `message:start` | `MessageStart` |
| `message:update` | `MessageDelta` (merged, not raw tokens) |
| `message:end` | `MessageEnd` |

### Tool Execution → Event Stream + HookSet

| pi Event | tau Mechanism |
|----------|--------------|
| `tool:start` | `ToolCallStart` (events) + `BeforeToolCall` chain |
| `tool:update` | `ToolCallUpdate` (events) |
| `tool:end` | `ToolCallEnd` (events) + `AfterToolCall` chain |

### Provider → HookSet + Callbacks

| pi Event | tau Mechanism |
|----------|--------------|
| `beforeProviderRequest` | `HookSet.BeforeProviderRequest` (LastWins) |
| `afterProviderResponse` | `StreamRequest.OnResponse` (per-request callback) |

### Session Lifecycle → Extension Runtime

| pi Event | tau Implementation |
|----------|-------------------|
| `session:start` | Fires on `NewCodingSession` |
| `session:shutdown` | Fires on `sess.Cancel()` |
| `session:beforeCompact` / `compact` | `HookSet.BeforeCompaction` (LastWins) |
| `session:beforeFork` / `tree` / `switch` | Product layer — Transcript is flat; branching on top |

### User Interaction & Resources → Product Layer

| pi Event | tau Implementation |
|----------|-------------------|
| `input` (slash commands) | `registerCommand` → CLI dispatch |
| `userBash` | `BeforeToolCall` filtered by `ToolName=="bash"` |
| `modelSelect` / `thinkingLevelSelect` | Product layer only |
| `resources:discover` | `registerTool`/`registerProvider` during init |
| `context` (dynamic injection) | `HookSet.TransformContext` chain |

**Coverage**: ~22 of 90+ events are core events. ~70 are TUI/UX (widgets, keybindings,
autocomplete, theming) — out of scope pending TUI phase.

---

## §3 — Hook vs Extension: Complementary

Hooks = **interception points** (core). Extensions = **distribution mechanism** (product layer).

| Scenario | Works? |
|----------|--------|
| Hooks without extensions | ✅ Go code calls `sess.Hooks.BeforeToolCall.Add(...)` directly |
| Extensions without hooks | ❌ Extensions need interception points to hook into |
| Extensions WITH hooks | ✅ The tau model — goja host objects register handlers on HookSet |

**Extensions consume hooks. They do not replace them.**

---

## §4 — Hot-Reload Strategy

Go cannot jiti-compile. Three strategies:

**Strategy A (v1): File Watcher + VM Recreate** — `fsnotify` on `.tau/extensions/` → kill
goja VM → create new VM → re-run all scripts. Extensions are stateless (state in Session).
Reload ~50ms for 10 extensions. In-flight tool calls lost on reload (acceptable).

**Strategy B**: No hot-reload — `tau --reload-extensions` or restart. Simpler.

**Strategy C (v2)**: Per-extension VM pool — only changed extensions reload.

**Decision: Strategy A for v1.** Degrade to B if watcher proves unreliable.

---

## §5 — Extension File Format

```javascript
// .tau/extensions/redraws.js
/// name: "tui-stats"
/// description: "TUI statistics slash command"

export default function(tau) {
    tau.registerCommand("tui-stats", {
        description: "Show TUI statistics",
        handler: function(args, ctx) { ctx.sendMessage("TUI stats: ..."); },
    });
    tau.on("tool:start", function(event, ctx) {
        if (event.toolName === "bash") { ctx.sendMessage("Running: " + event.args.command); }
    });
}
```

Frontmatter (`/// name:`, `/// description:`) is optional — parsed for `tau extensions list`.

---

## §6 — Go API Draft (Host Objects)

```go
// pkg/coding/extensions/api.go

// ExtensionAPI — exposed to goja scripts as global `tau`.
type ExtensionAPI struct {
    On               func(eventName string, handler goja.Callable)
    RegisterTool     func(def map[string]interface{}) // {name, description, schema, handler}
    RegisterCommand  func(name string, def map[string]interface{})
    RegisterProvider func(name string, cfg map[string]interface{})
}

// ExtensionContext — second arg to event/command handlers.
type ExtensionContext struct {
    SessionID       string
    SendMessage     func(text string)
    SendUserMessage func(text string)
    GetActiveTools  func() []string
    SetActiveTools  func(names []string)
    NewSession      func(opts map[string]interface{}) (ExtensionContext, error)
    Fork            func() (ExtensionContext, error)
    SwitchSession   func(id string) (ExtensionContext, error)
    Reload          func() error
}
```

### Stale-Context Safety (pi pattern preserved)

After `ctx.NewSession()`, `Fork()`, `SwitchSession()`, or `Reload()`, the old `ctx` becomes
**stale** — all methods throw: "This extension context is stale after session replacement."

Implementation: each context has `active bool`; session-replacement sets `active=false` on
old context before returning new one.

### Pre-Bind Queueing (pi pattern preserved)

Registrations (`registerTool`, `registerProvider`) during load are queued, flushed when core
session ready. Action methods before binding return: "Extension not yet bound to session."

---

## §7 — Routing Table

| Extension Event | Core Mechanism | Category |
|----------------|---------------|----------|
| `turn:start` / `turn:end` | `run.Events` → filter `TurnStart`/`TurnEnd` | Event stream |
| `message:start` / `update` / `end` | `run.Events` → filter `MessageStart`/`Delta`/`End` | Event stream |
| `tool:start` | `ToolCallStart` + `BeforeToolCall` chain | Event + Hook |
| `tool:update` | `ToolCallUpdate` | Event stream |
| `tool:end` | `ToolCallEnd` + `AfterToolCall` chain | Event + Hook |
| `beforeProviderRequest` | `HookSet.BeforeProviderRequest` (LastWins) | Hook |
| `afterProviderResponse` | `StreamRequest.OnResponse` callback | Hook |
| `beforeCompaction` | `HookSet.BeforeCompaction` (LastWins) | Hook |
| `session:start` / `shutdown` | Extension runtime on `NewCodingSession`/`Cancel()` | Product |
| `input` (slash commands) | `registerCommand` → CLI dispatch | Product |
| `userBash` | `BeforeToolCall`, filter `ToolName=="bash"` | Hook |
| `context` (dynamic injection) | `HookSet.TransformContext` chain | Hook |

**17 trigger points, zero new core mechanisms.** Remaining ~70 pi events are TUI/UX — deferred.

---

## §8 — Package Layout

```
pkg/coding/extensions/
  api.go, context.go, loader.go, watcher.go, queue.go, events.go, commands.go, sandbox.go
```

---

## §9 — Sandbox (v1)

goja VM: no network (`fetch`/`XMLHttpRequest` not exposed), no filesystem (`fs` not exposed),
CPU limit 5s per handler via goja `Interrupt`, no `eval`. Extensions are **trusted user scripts**
(pi model) — sandbox is safety net, not security boundary.

---

## §10 — Out of Scope (v1)

Subprocess/Go-native extensions (v2), Wasm sandbox (v2+), TUI widget/keybinding extensions
(TUI phase), extension marketplace/signing (Never core / v2), per-extension VM pool (v2).

---

## §11 — Core Gaps

**None.** All extension capabilities map to existing abstractions:
`run.Events` + `HookSet.*` + `StreamRequest.OnResponse` + `core.NewSession` + `sess.Cancel()`.

**Zero core source file modifications required.** goja VM lives entirely in `pkg/coding/extensions/`.

---

*11-extension-system.md — goja runtime, 17 mapped events, Hook+Extension complementary, hot-reload via fsnotify, zero core changes.*