# 1. Agent Loop

Source: `packages/agent/src/agent-loop.ts` (742 LOC), `packages/agent/src/agent.ts` (557 LOC), `packages/agent/src/types.ts` (418 LOC).

## How it works

The loop is purely functional over a `Context` (system prompt + messages + tools), driven by an **event sink** rather than a return value.

Two entry points:
- `runAgentLoop(prompts, ctx, cfg, emit, signal, streamFn)` — start with new prompts.
- `runAgentLoopContinue(ctx, cfg, ...)` — resume from a `user`/`toolResult` tail (e.g., for retries).

Both call the same private `runLoop`.

The loop is a **double `while`**:

- **Inner loop (turn loop)**:
  1. Emit `turn_start`.
  2. Drain pending steering messages (inject into context as user/custom messages).
  3. Call `streamAssistantResponse` → produces an `AssistantMessage` (with optional tool calls).
  4. Execute tool calls (sequentially or in parallel — see [tool protocol](03-tool-protocol.md)).
  5. Emit `turn_end`.
  6. Optionally call `prepareNextTurn` to swap context/model/thinking-level.
  7. Optionally call `shouldStopAfterTurn`.
  8. Re-poll steering messages.
  9. Continue while `hasMoreToolCalls || pendingMessages.length > 0`.

- **Outer loop (follow-up loop)**:
  1. When the inner loop runs out of work, call `getFollowUpMessages`.
  2. If anything is queued, those become the next batch and we re-enter the inner loop.
  3. Otherwise: emit `agent_end`, return.

## Termination model — explicit and multi-cause, NOT a turn counter

- `stopReason === "error" | "aborted"` → emit `turn_end` + `agent_end`, return.
- No tool calls **and** no steering **and** no follow-up → exit.
- `shouldStopAfterTurn` returns `true` → exit (the harness uses this to stop *before* the next provider call when the context is near full).
- Every finalized tool result has `terminate: true` → batch sets `hasMoreToolCalls = false`.
- **No token-budget cap. No max-turn cap.** The loop runs until something says stop.

This is a deliberate design choice: all budget enforcement lives in hooks (the harness's responsibility), keeping the loop pure.

## Tool-result feedback

Tool results are appended **directly to the live `currentContext.messages` array** (and to the `newMessages` accumulator that becomes the loop's return value).

The next iteration's `streamAssistantResponse` sees them automatically. There is no separate "feed result back to LLM" step — the context is live and mutated in place.

## Agent ↔ Harness contract

The crucial split:

- **The agent loop** is stateless about persistence and provider auth. It only knows these callbacks (defined in `AgentLoopConfig`):
  - `convertToLlm` — `AgentMessage[]` → `Message[]` for the wire format.
  - `transformContext` — last-mile context transformation (e.g., context-window pruning).
  - `getApiKey` — per-call auth resolution (designed for short-lived OAuth tokens).
  - `beforeToolCall` / `afterToolCall` — tool execution hooks.
  - `prepareNextTurn` — swap context/model/thinking before the next provider call.
  - `shouldStopAfterTurn` — graceful stop hook.
  - `getSteeringMessages` / `getFollowUpMessages` — queue drains.

- **The harness** wires those callbacks to a `Session` (storage), a hook bus, and a `streamFn` factory that injects auth + payload-transform hooks.

- The loop never imports the harness; the harness composes the loop. **Everything is callbacks.**

## Streaming

Streaming is the **only** path. `streamAssistantResponse` consumes a sequence of provider events:

```
start → (text|thinking|toolcall)_(start|delta|end)* → done | error
```

- Each non-terminal event re-emits a `message_update` with a fresh shallow clone of the partial assistant message.
- The final `done` or `error` event delivers the canonical `AssistantMessage` via `response.result()`.
- Non-streaming consumers just `await stream.result()`.

The provider stream contract (declared at `types.ts:StreamFn`) is strict:

> **Failures must be encoded in the stream as a final `error` event with an `AssistantMessage` carrying `stopReason: "error"|"aborted"` + `errorMessage` — never thrown.**

That's why the agent loop has almost no `try/catch` around provider calls. Errors are values.

## Go sketch

```go
type AgentLoop struct {
    Stream       StreamFn
    ConvertToLLM func([]Message) []LLMMessage
    Hooks        Hooks
    Queues       Queues
}

type Hooks struct {
    TransformContext    func(ctx context.Context, msgs []Message) ([]Message, error)
    BeforeToolCall      func(ctx context.Context, tc ToolCall) (block bool, reason string, err error)
    AfterToolCall       func(ctx context.Context, tc ToolCall, res ToolResult) (ToolResult, error)
    PrepareNextTurn     func(ctx context.Context) (NextTurnUpdate, error)
    ShouldStopAfterTurn func(ctx context.Context) (bool, error)
}

func (a *AgentLoop) Run(
    ctx context.Context,
    prompts []Message,
    agentCtx AgentCtx,
    emit EventSink,
) ([]Message, error)
```

## What's elegant

- **Callback-driven extension**: `prepareNextTurn` lets the harness swap *the entire context* between turns without the loop knowing about sessions or storage.
- **Two-queue steering model**: steering messages inject *between* assistant turns; follow-up messages inject *after the agent would otherwise stop*. Maps cleanly onto interactive UX (interrupt vs schedule-after).
- **Live-context mutation**: tool results land in `currentContext.messages` immediately. No "build the next request payload" step.
- **`terminate` only honored when ALL tools agree**: prevents one tool from forcing exit when others want to continue. See `shouldTerminateToolBatch`.

## TS-specific (skip in Go)

- `AsyncIterable` machinery → use Go channels (`<-chan StreamEvent`).
- Per-event `{...partial}` shallow-clone for streaming snapshots → pass values directly or use immutable builders.
- `declare module` for extending `CustomAgentMessages` → use a sealed union (interface with private tag method) in Go.