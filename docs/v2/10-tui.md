# 10 — Terminal UI

> **Library**: Bubble Tea (charmbracelet/bubbletea v1) — Elm-like TUI framework.
> **Architecture**: Model/Update/View, goroutine bridge for agent events.
> **Zero core changes**.

## Layout

```
┌─────────────────────────────────┐
│ Messages (scrollable)           │
│ You: hello                      │
│ tau: I'll help with...          │
│   [bash] → ECHO: hello          │
├─────────────────────────────────┤
│ [idle]                          │
├─────────────────────────────────┤
│ > _                             │
└─────────────────────────────────┘
```

## Model State

```go
type model struct {
    session   *coding.CodingSession
    loop      core.Loop
    messages  []line              // rendered message lines
    streaming string              // current assistant delta
    tools     map[string]toolState // in-flight tool calls
    input     textinput.Model     // user input field
    status    string              // idle | streaming | tool | done
    width, height int
    agentChan chan any            // bridge: agent goroutine → TUI (AgentEvent | turnCompleteMsg)
    err       error
}

line = {role, content}
toolState = {name, status: running|done, result}
```

## Event Flow (goroutine bridge)

1. User submits input → goroutine runs turn loop (Prompt → events → Continue → events → …)
2. All AgentEvents pumped into `agentChan`
3. `listenEvents` Cmd reads from `agentChan`, wraps in `eventMsg` for Update
4. When turn loop finishes, goroutine sends `turnCompleteMsg` → Update sets status=idle
5. Keyboard: Ctrl+C → quit, Enter → submit, Esc → quit

## Key Design Decisions
- Single goroutine runs full Prompt+Continue loop; all events through one channel
- `agentChan chan any` carries both AgentEvent and turnCompleteMsg (type-switch in Update)
- textinput for readline-style input
- Simple append-only scroll with window-based clipping
- No paging — messages scrolling only
