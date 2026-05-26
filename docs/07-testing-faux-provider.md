# 7. Testing — the Faux Provider Pattern

Source: `packages/ai/src/providers/faux.ts` (499 LOC).

This is the testing pattern that makes pi's tests fast, deterministic, and *meaningful*. **It is directly portable to Go and is high-leverage.**

## 7.1 What it is

`packages/ai/src/providers/faux.ts` is a **complete in-memory provider** registered just like a real one. Used by every harness/agent test in the repo.

```typescript
const faux = registerFauxProvider({
    api: "faux-test",
    models: [{id: "faux-1", contextWindow: 128000}],
});

faux.setResponses([
    fauxAssistantMessage("Hello"),
    fauxAssistantMessage([fauxToolCall("read", {path: "/tmp/x"})]),
    fauxAssistantMessage("done", { stopReason: "stop" }),
    // dynamic factory:
    (context, options, state, model) => fauxAssistantMessage("call #" + state.callCount),
]);

const harness = new AgentHarness({
    model: faux.getModel(),
    session: ...,
    ...
});

await harness.prompt("hi");
```

That's the whole pattern. The harness, agent loop, compaction, tool execution all run **for real** against canned LLM responses.

## 7.2 Why it works

**The faux provider implements the same `StreamFunction` contract as Anthropic, OpenAI, etc.** It registers in the same `apiProviderRegistry`. From the harness's point of view, it is indistinguishable from a real provider.

This means:
- The agent loop runs unchanged.
- The harness runs unchanged.
- Compaction runs unchanged (and you can simulate token usage to trigger it!).
- Tool execution runs unchanged.
- Hook bus runs unchanged.

**There is NO mock layer above the provider.** Tests exercise the *real* code paths.

## 7.3 Key features

### Pre-queued response sequence

```typescript
faux.setResponses([msg1, msg2, msg3]);
faux.appendResponses([msg4, msg5]);
faux.getPendingResponseCount();
```

Each LLM call shifts one `FauxResponseStep` off the queue. If empty when called → emits an error message: `"No more faux responses queued"`. This makes "did the agent make exactly N calls?" a passive assertion — over-call fails noisily.

### Streaming with realistic deltas

`streamWithDeltas` chunks the canned message into `text_delta` / `toolcall_delta` events sized by random `[minTokenSize, maxTokenSize]`, with optional `tokensPerSecond` pacing. So:

- UI tests see realistic incremental updates.
- Race conditions between deltas and abort/steer are reachable.
- "Real-feel" tests run with `tokensPerSecond: 30` to pace like a real model.

### Usage estimation with prompt-cache simulation

This is the killer feature. `withUsageEstimate`:

```typescript
function withUsageEstimate(message, context, options, promptCache) {
    const promptText = serializeContext(context);
    const promptTokens = estimateTokens(promptText);
    const outputTokens = estimateTokens(...);

    if (sessionId && cacheRetention !== "none") {
        const previousPrompt = promptCache.get(sessionId);
        if (previousPrompt) {
            const cachedChars = commonPrefixLength(previousPrompt, promptText);
            cacheRead = estimateTokens(previousPrompt.slice(0, cachedChars));
            cacheWrite = estimateTokens(promptText.slice(cachedChars));
            input = Math.max(0, promptTokens - cacheRead);
        } else {
            cacheWrite = promptTokens;
        }
        promptCache.set(sessionId, promptText);
    }
    return { ...message, usage: { input, output, cacheRead, cacheWrite, ... }};
}
```

**Prompt caching behavior is testable without a real provider.** You can verify:
- That `sessionId` is propagated correctly.
- That cache hits grow as the conversation grows.
- That a prompt-prefix change invalidates the cache.

### Factory responses

A response step can be a function:

```typescript
(context, options, state, model) => AssistantMessage
```

It receives:
- `context`: full LLM-converted context (so you can assert on what the agent sent).
- `options`: stream options (so you can verify cache retention, headers, etc.).
- `state.callCount`: the call number (so you can branch on "first call vs subsequent").
- `model`: the model object.

This lets tests assert-and-respond:

```typescript
faux.setResponses([
    (context) => {
        const lastMsg = context.messages[context.messages.length - 1];
        if (containsToolResultFor(lastMsg, "read")) {
            return fauxAssistantMessage("Got the file content. Now let me edit it.");
        }
        return fauxAssistantMessage("Unexpected state");
    },
]);
```

### Abort propagation

`streamWithDeltas` checks `signal.aborted` between chunks and emits a synthetic aborted message. So **abort tests are reliable** without timing flakiness.

### Lifecycle hooks fire normally

`onResponse` and `onPayload` are invoked with synthetic `{status: 200, headers: {}}`. So the harness's `before_provider_payload` hook is testable end-to-end without a real network round-trip.

## 7.4 Why this beats traditional mocking

Traditional mocking pattern:
```typescript
jest.mock("./agent-loop");
mockAgentLoop.mockResolvedValue([...messages]);
```

Problems:
- Mocked behavior diverges from real behavior over time.
- Bug in agent-loop won't surface in tests that mock it.
- Coverage looks high but isn't meaningful.

Faux-provider pattern:
- The agent-loop, harness, compaction code is the **real** code under test.
- Only the LLM HTTP boundary is fake.
- You can refactor the agent-loop and tests still cover its behavior.

This is why the AGENTS.md rule says:

> For `packages/coding-agent/test/suite/`, use `test/suite/harness.ts` + the faux provider. **No real provider APIs, keys, or paid tokens.**

## 7.5 Go port

Directly portable. Skeleton:

```go
package faux

type Provider struct {
    api          string
    pendingResps []ResponseStep
    state        *State
    promptCache  map[string]string  // sessionID → last prompt text
}

type State struct {
    CallCount int
}

type ResponseStep interface { isResponseStep() }

type StaticResponse  struct { Msg *AssistantMessage }
type FactoryResponse struct {
    Fn func(ctx context.Context, model Model, reqCtx Context, opts StreamOptions, state *State) *AssistantMessage
}

func Register(opts RegisterOpts) *Provider { ... }
func (p *Provider) SetResponses(steps []ResponseStep) { ... }
func (p *Provider) AppendResponses(steps []ResponseStep) { ... }

func (p *Provider) Stream(ctx context.Context, model Model, reqCtx Context, opts StreamOptions) (<-chan StreamEvent, error) {
    ch := make(chan StreamEvent, 16)
    go func() {
        defer close(ch)
        p.state.CallCount++

        if len(p.pendingResps) == 0 {
            // emit error event
            return
        }
        step := p.pendingResps[0]
        p.pendingResps = p.pendingResps[1:]

        msg := resolveStep(step, ctx, model, reqCtx, opts, p.state)
        msg = withUsageEstimate(msg, reqCtx, opts, p.promptCache)
        streamWithDeltas(ch, msg, ctx)
    }()
    return ch, nil
}
```

Add the prompt-cache simulator (common-prefix matching against previous calls with the same `sessionId`). Test compaction by queueing responses with realistic `Usage{}` values that drive `shouldCompact()`.

## 7.6 Testing patterns this enables

Once you have a faux provider, you can write tests like:

- **"Compaction triggers at 90% context window"**: queue responses with usage values that ramp up, assert that compaction fires at the right turn.
- **"Steer message gets injected after current turn finishes"**: queue tool-calling response → during the (synthetic) tool execution, call `harness.steer("update")` → assert next response sees the steer.
- **"Tool error doesn't crash the loop"**: queue a tool call for a tool that throws → assert next response sees a tool result with `isError: true`.
- **"Abort during streaming returns aborted message"**: queue long response with `tokensPerSecond: 100` → call `harness.abort()` mid-stream → assert final stopReason is `aborted`.
- **"Prompt cache hit grows with conversation"**: track `usage.cacheRead` across turns, assert it grows monotonically.
- **"Provider-supplied compaction (via hook) skips the LLM call"**: register a `session_before_compact` hook that returns a canned summary → assert the faux provider's `callCount` does not increase during `harness.compact()`.

This test surface — covering real harness logic with deterministic outputs — is the single highest-leverage thing you can build first in the Go port.