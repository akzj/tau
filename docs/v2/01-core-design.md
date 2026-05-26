# 01 — Core Design

> **Scope**: The six abstractions forming tau's agent runtime kernel.
> **Basis**: `04-design-principles.md` (R1–R6, P1+P2). Next: `02-provider.md`.

---

## §1 — Loop

```go
type Loop interface {
    Prompt(ctx context.Context, sess *Session, input UserInput) (*Run, error)
    Continue(ctx context.Context, sess *Session) (*Run, error)  // internal
}
type Run struct {
    Events <-chan AgentEvent   // high-level semantic, NOT raw provider tokens
    Done   func() (RunResult, error)
    Cancel func()              // cooperative; tools/providers honor ctx.Done()
}
```

- **Stateless** (C6+C1): state → Session; Loop is a pure function runner. Two entries (not four — pi's `steer`/`followUp` deferred to Phase 5+ per A2 #4).
- **Double-layer stream** (A2 #2): Loop consumes raw `ProviderEvent` internally → emits semantic `AgentEvent` to product layer.
- **Cancel is cooperative** (A1 §3): `ctx.Done()` signals; in-flight `Execute` not forcibly killed.

---

## §2 — Session

```go
type Session struct {
    ID           SessionID
    Transcript   *Transcript
    Tools        *ToolRegistry
    Providers    *ProviderRegistry
    Hooks        *HookSet
    SystemPrompt SystemPromptFn    // function, NOT mutable string
}
type SystemPromptFn func(sess *Session) (string, error)
```

- **Isolation boundary** (C1/R4). All mutable state lives here. No `DefaultSession` or `GlobalSession`.
- **`SystemPrompt` is a function** (A2 #7): kills the `agent.state.systemPrompt = X` mutation anti-pattern. Dynamic needs (skills from disk) use closure over external state.
- **Context scoping**: `Loop.Prompt` derives child ctx. `Session.Cancel()` → session scope; `Run.Cancel()` → run scope.
- **Scope discipline**: only per-session mutable state. Behavior params → `LoopOptions`/`ProviderOptions`. Cross-session references → package-level `var`.

---

## §3 — Transcript

```go
type Transcript struct { /* hidden */ }
func (t *Transcript) Append(msgs ...Message) (Position, error)
func (t *Transcript) Slice(from, to Position) []Message          // snapshot, non-blocking
func (t *Transcript) Subscribe(cursor Position) (<-chan Message, func())
type Position int64
```

- **Flat, not tree** (design note). Linear append-only log. Fork/branch/undo/time-travel are product-layer concerns — build a tree view on top, not inside. Pi's `LeafEntry` is the reference for that product layer.
- **No internal mirror** (C6). Loop reads from `Transcript.Slice`, never from a cached copy.
- **`Subscribe`** bridges to TUI/WebSocket consumers (see `05-access-points.md`).

---

## §4 — AgentEvent

```go
type AgentEvent interface { eventMarker(); Timestamp() time.Time }  // sealed

type MessageStart   struct{ MessageID string; Role Role }
type MessageDelta   struct{ MessageID string; ContentDelta string }
type MessageEnd     struct{ MessageID string }
type ToolCallStart  struct{ CallID, ToolName string; Args json.RawMessage }
type ToolCallUpdate struct{ CallID string; Partial PartialResult }
type ToolCallEnd    struct{ CallID string; Result ToolResult }
type TurnStart      struct{ TurnID string }
type TurnEnd        struct{ TurnID string; Reason TurnEndReason }
type ErrorEvent     struct{ Err error; Code ErrorCode }
```

- **Sealed interface**: `eventMarker()` private → compiler-exhaustive type-switch. New event types require core-package change → enforced design review (C2/R6).
- **Not raw `ProviderEvent`** (A2 #3). Token deltas consumed inside Loop; product sees merged `MessageDelta`.
- **`ToolCallUpdate` caveat**: parallel batches may reorder partials. Product layers needing strict ordering should self-sort by `CallID + seq`.

---

## §5 — Tool

```go
type Tool struct {
    Name, Description string
    Schema             ToolSchema
    Execute            ToolExecuteFn
    PrepareArgs        func(raw json.RawMessage) (any, error)  // optional
    Mode               ExecutionMode                            // Sequential | Parallel
}
type ToolExecuteFn func(ctx context.Context, callID string, params any,
    onUpdate func(PartialResult)) (ToolResult, error)  // callback, NOT channel (A1 §3)

type ToolResult struct {
    Content   []Content; Details any; Terminate bool
}
type ToolRegistry struct{ /* ... */ }
func (r *ToolRegistry) Register(t Tool) error
func (r *ToolRegistry) Get(name string) (Tool, bool)
func (r *ToolRegistry) Active() []Tool
func (r *ToolRegistry) SetActive(names []string)
```

- **Single canonical Tool** (C4). Product layers wrap with UI/prompt fields, degrade to `Tool`.
- **`onUpdate` callback, not channel**: channels complicate close/buffer/ctx coordination. Partials ≤10/s — callback sufficient (Phase 5+ revisit if ≥100/s).
- **Error → `return err`** (not encoded in Content). **`Terminate` → hint**; only triggers when entire batch agrees.
- **`PrepareArgs` optional**: compatibility valve for older models emitting raw-string args.

---

## §6 — Hook

```go
type Chain[T any] interface {
    Add(handler func(context.Context, T) (T, error))
    Run(ctx context.Context, init T) (T, error)
}
type LastWins[T any] interface {
    Set(handler func(context.Context, T) (*T, error))   // duplicate → panic (P2)
    Observe(handler func(context.Context, T))           // read-only
    Run(ctx context.Context, init T) (T, error)
}
type HookSet struct {
    BeforeProviderRequest LastWins[StreamRequest]
    TransformContext      Chain[[]Message]
    BeforeToolCall        Chain[ToolCallEvent]
    AfterToolCall         Chain[ToolResultEvent]
}
```

- **Two distinct interfaces** (C5). `Chain.Add` vs `LastWins.Set`/`Observe`. Compiler catches mis-registration.
- **`LastWins.Set` duplicate → panic** (P2 panic-fast): init-phase config bug.
- **Fields driven by product-layer use cases** (A2 11 access points). G4 路径A adds `BeforeCompaction LastWins[CompactionRequest]` — see `06-compaction.md`.

---

## §7 — Skills (Deferred)

Skills are **not a first-class core abstraction** (G3). Pi's coding-agent assembles skills via system prompt concatenation; agent-core is transparent. Tau's `SystemPromptFn` closure already supports `load SKILL.md → format → return string`. No behavioral data on LLM proactive SKILL.md reading (B8). Phase 5+ may revisit.

---

## §8 — Connection Points

| This file → | Why |
|-------------|-----|
| `02-provider.md` | `ProviderRegistry` on Session; `StreamRequest` consumed by Loop. |
| `03-tool-schema.md` | `Tool.Schema` implements `ToolSchema` — schema-first codegen. |
| `05-access-points.md` | Every access point surfaces on these abstractions. |
| `06-compaction.md` | `HookSet` + `BeforeCompaction` (G4 路径A). |
| `07-go-structure.md` | All types → `core/` package. |

---

*01-core-design.md — six abstractions, one kernel.*
