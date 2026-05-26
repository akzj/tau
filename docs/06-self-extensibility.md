# 6. Self-extensibility

Source: `packages/coding-agent/src/core/extensions/` (loader 600 LOC, runner 1068 LOC, types 1567 LOC, index 172 LOC).

The `README.md` advertises pi as "our self extensible coding agent". This refers to the `.pi/extensions/` mechanism — **TypeScript files dropped into a directory that get hot-loaded and gain access to the agent's full hook surface.**

## 6.1 What an extension looks like

Real example from this repo (`.pi/extensions/redraws.ts`):

```typescript
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Text } from "@earendil-works/pi-tui";

export default function (pi: ExtensionAPI) {
    pi.registerCommand("tui", {
        description: "Show TUI stats",
        handler: async (_args, ctx) => {
            if (!ctx.hasUI) return;
            let redraws = 0;
            await ctx.ui.custom<void>((tui, _theme, _keybindings, done) => {
                redraws = tui.fullRedraws;
                done(undefined);
                return new Text("", 0, 0);
            });
            ctx.ui.notify(`TUI full redraws: ${redraws}`, "info");
        },
    });
}
```

Each extension exports a default function `(pi: ExtensionAPI) => void` (sync or async). The function registers handlers, tools, commands, providers, editors, widgets, keybindings.

## 6.2 Loading: jiti + virtual modules

This is the clever bit. Pi has two runtime modes:

1. **Dev / Node mode**: extensions are `.ts` files that need to be runtime-compiled.
2. **Compiled Bun binary mode**: extensions are still `.ts` files, but at runtime they need to import `@earendil-works/pi-agent-core` etc., and those packages are not on disk — they're inside the binary.

Pi uses **`jiti`** for the runtime TS compiler, with two key tricks:

### Trick 1: Static imports force Bun to bundle dependencies

```typescript
// Static imports — these MUST be static so Bun bundles them into the compiled binary.
import * as _bundledPiAgentCore from "@earendil-works/pi-agent-core";
import * as _bundledPiAi from "@earendil-works/pi-ai";
import * as _bundledPiTui from "@earendil-works/pi-tui";
import * as _bundledTypebox from "typebox";
// ...
```

These imports look unused, but they exist purely so the bundler includes those packages in the compiled binary.

### Trick 2: jiti `virtualModules` for in-binary resolution

```typescript
const VIRTUAL_MODULES: Record<string, unknown> = {
    "typebox": _bundledTypebox,
    "@sinclair/typebox": _bundledTypebox,
    "@earendil-works/pi-agent-core": _bundledPiAgentCore,
    "@earendil-works/pi-tui":        _bundledPiTui,
    "@earendil-works/pi-ai":         _bundledPiAi,
    "@earendil-works/pi-coding-agent": _bundledPiCodingAgent,
    // legacy aliases:
    "@mariozechner/pi-agent-core":   _bundledPiAgentCore,
    "@mariozechner/pi-tui":          _bundledPiTui,
    // ...
};
```

When an extension does `import "@earendil-works/pi-agent-core"`, jiti is configured with `virtualModules` to satisfy that resolution from the bundled in-memory module instead of the filesystem.

### Result

- **One extension source file** works in both dev (Node + jiti hot-load from disk) and production (compiled Bun binary + virtualModules).
- **No precompile step** for extensions.
- **Aliases also map both `@earendil-works/*` and the legacy `@mariozechner/*` namespace** for backwards compat.

## 6.3 What extensions can do — the `ExtensionAPI`

Inspecting `packages/coding-agent/src/core/extensions/index.ts` exports, extensions can:

### Event subscription (90+ event types)
- `pi.on(event, handler)` for: agent lifecycle, message lifecycle, turn lifecycle, tool execution, session lifecycle (start/end/compact/tree/fork/switch), provider request/response, input events, resources discovery, model/thinking-level changes, etc.
- Many events allow returning a **patch** that mutates the flow (chain-of-responsibility pattern, same as the harness hook bus).

### Registrations
- `pi.registerTool(toolDef)` — add a typed tool to the active set.
- `pi.registerCommand(name, {description, handler})` — slash command with shell-style arg parsing.
- `pi.registerProvider(name, providerConfig)` — register a custom AI provider at runtime (queued during load, flushed on `bindCore()`).
- `pi.registerEditor(...)`, `pi.registerWidget(...)`, `pi.registerKeybinding(...)` — TUI hooks.
- `pi.registerMessageRenderer(...)` — custom message rendering.
- `pi.registerAutocompleteProvider(...)` — input autocomplete.

### Action methods on `ctx`
- `ctx.sendMessage`, `ctx.sendUserMessage`, `ctx.appendEntry`.
- `ctx.setSessionName`, `ctx.setLabel`.
- `ctx.getActiveTools`, `ctx.getAllTools`, `ctx.setActiveTools`.
- `ctx.setModel`, `ctx.getThinkingLevel`, `ctx.setThinkingLevel`.
- `ctx.fork`, `ctx.newSession`, `ctx.switchSession`, `ctx.reload`.
- `ctx.exec` — run shell commands.

## 6.4 Sandboxing: NONE

Extensions run **in-process with full filesystem and network access**. The "safety" is convention: extensions are user-installed code, treated like dotfile config. The TS-only loader does enforce that virtualModules are read-only references to bundled internals.

This is intentional: pi's extension model is "trusted user scripts," not "untrusted plugin marketplace."

## 6.5 Stale-context safety

A subtle but important detail (`createExtensionRuntime` in `loader.ts`):

After `ctx.newSession()` / `ctx.fork()` / `ctx.switchSession()` / `ctx.reload()`, the captured `ctx` and the captured `pi` API become **stale**. The runtime calls `runtime.invalidate(message)` which makes `assertActive()` throw on every API method.

The error message is explicit:

> "This extension ctx is stale after session replacement or reload. Do not use a captured pi or command ctx after ctx.newSession(), ctx.fork(), ctx.switchSession(), or ctx.reload(). For newSession, fork, and switchSession, move post-replacement work into withSession and use the ctx passed to withSession. For reload, do not use the old ctx after await ctx.reload()."

**Invalidate-and-throw beats silent staleness.** This is a pattern worth porting.

## 6.6 Go port options

A pure Go agent system **cannot use jiti-style hot TS loading**. Realistic options:

### Option A: Embedded scripting (RECOMMENDED)

Pick one:
- **Starlark** (`go.starlark.net`) — Python-flavored, deterministic, no I/O by default. Best for safety-critical extensions.
- **goja** (`github.com/dop251/goja`) — JavaScript (ES5.1+ subset, ES6 partial). Best familiarity for TS users porting their pi extensions.
- **Lua** via gopher-lua — small footprint, simple.

Define `ExtensionAPI` as a host object exposed to the script:

```go
type Runtime struct {
    vm *goja.Runtime
}

func (r *Runtime) LoadExtension(path string) (*Extension, error) {
    src, _ := os.ReadFile(path)
    api := buildExtensionAPI()  // host object
    r.vm.Set("pi", api)
    _, err := r.vm.RunScript(path, string(src))
    return ext, err
}
```

Pros: in-process (fast IPC), sandboxable, no separate runtime to ship.
Cons: not native Go — performance overhead for hot tool callbacks.

### Option B: Subprocess

Extension = separate Go binary speaking JSON over stdio:

```
{"type": "register_command", "name": "foo", ...}
{"type": "command_invoke", "name": "foo", "args": [...]}
{"type": "command_result", "ok": true, ...}
```

Pros: clean isolation, OS-portable, language-agnostic (you can write extensions in any language).
Cons: per-extension startup cost; serialization overhead on every callback.

### Option C: Wasm via wazero

Pros: real sandbox, deterministic, future-proof.
Cons: more upfront work; smaller ecosystem; tool authors need to understand wasm.

### Option D: Go `plugin` package — AVOID

UNIX-only, version-fragile (must match exact Go version + CGO settings of the host), can't be reloaded, can't be sandboxed. Don't.

## 6.7 What to keep regardless of backend

Whichever runtime you pick, preserve these design properties from pi:

1. **Extension shape**: `(api) => void` (or `(api) -> None` in Starlark). Simple registration model.
2. **`invalidate(message)` for stale-context safety.** When a session swap happens, captured handles must throw on use.
3. **Pre-bind queueing for things that need core to be ready** (pi queues `pendingProviderRegistrations` during load and flushes on `bindCore()`). Mirror this pattern: registration during load is fine, but action methods on `ctx` should fail if called too early.
4. **Two-tier API**: registration methods (write to extension state) vs action methods (delegate to runtime). Pi separates these clearly.