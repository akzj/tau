# 3. Tool Protocol

Source: `packages/agent/src/types.ts` (tool types), `packages/agent/src/agent-loop.ts` (execution model).

## Tool definition shape

```typescript
interface AgentTool<TParameters extends TSchema, TDetails> extends Tool<TParameters> {
    name: string;
    label: string;          // UI-only display label
    description: string;    // model-visible
    parameters: TSchema;    // typebox schema → JSON schema for the model
    prepareArguments?(raw: unknown): Static<TParameters>;  // pre-validation shim
    execute(toolCallId, params, signal?, onUpdate?): Promise<AgentToolResult<TDetails>>;
    executionMode?: "sequential" | "parallel";  // per-tool override
}

interface AgentToolResult<TDetails> {
    content: (TextContent | ImageContent)[];   // returned to model
    details: TDetails;                          // for UI / logs / NOT sent to model
    terminate?: boolean;                        // hint to stop after batch
}
```

The **`details`** field is a critical design choice: structured data (e.g., `{exitCode, fullOutputPath}`) carried alongside the model-visible `content` for UI rendering and post-hoc analysis, **never serialized into the LLM context**.

## Registration

There is **no global registry**. Tools live on the `AgentHarness.tools` map (or `AgentContext.tools`).

- `setTools(tools, activeToolNames?)` — replaces the map and (re)validates active tool names.
- `setActiveTools(names)` — changes the model-visible subset without changing the catalog.

This means: **the model sees only `activeTools`, but extensions can define many tools and toggle which are exposed.**

## Execution model

The loop supports **both modes** with a per-tool override:

- **`sequential`** — prepare + execute + finalize each call before the next.
- **`parallel`** — *prepare* all calls sequentially (so `beforeToolCall` hooks see them in order), then `Promise.all` the executions. `tool_execution_end` events fire in completion order; the **resulting `toolResult` messages are emitted in source order** (preserving conversation determinism).
- If any tool in the batch is marked `executionMode: "sequential"`, the whole batch falls back to sequential.

Tools may stream partial updates via the `onUpdate` callback → `tool_execution_update` events.

## Error handling

Each tool call goes through three phases (`agent-loop.ts`):

1. **prepare** (`prepareToolCall`):
   - Tool lookup by name (missing → immediate error).
   - `prepareArguments` shim (raw-args compatibility for older models).
   - `validateToolArguments` (typebox schema check).
   - `beforeToolCall` hook (can `block` with reason).
   - Failures here yield an `immediate` outcome with `kind:"immediate"` and an error tool result — execution is skipped.

2. **execute** (`executePreparedToolCall`):
   - Wraps `tool.execute` in `try/catch`. Throws → error tool result with `isError: true`. Success → `executed` outcome.

3. **finalize** (`finalizeExecutedToolCall`):
   - Runs `afterToolCall` hook, applies its patch (`content`/`details`/`isError`/`terminate`) field-by-field. Hook throws → replaces result with an error.

**Errors are values** (`AgentToolResult` with `isError: true`); they never crash the loop. The tool result message is always emitted.

## Built-in tools

The `agent` package itself ships **no built-in tools**.

The harness provides a `Shell` and `FileSystem` capability via `ExecutionEnv`, but tools that use them (read/write/edit/bash/grep/find/ls) live in `packages/coding-agent`. **The agent core is a runtime, not an application.**

The **convention** is that tool names like `read`, `write`, `edit` get scanned by `extractFileOpsFromMessage` (in `compaction/utils.ts`) for `args.path` to populate compaction's `FileOperations`. Cross-cutting bookkeeping is **convention-based, not schema-enforced**.

## Go sketch

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() *jsonschema.Schema
    Execute(
        ctx context.Context,
        callID string,
        args json.RawMessage,
        onUpdate func(ToolResult),
    ) (ToolResult, error)
    ExecutionMode() ExecutionMode  // sequential / parallel / "" (default)
}

type ToolResult struct {
    Content   []ContentBlock  // sent to LLM
    Details   any             // for UI, never to LLM
    Terminate bool
    IsError   bool
}

type ExecutionMode string

const (
    ExecutionModeDefault    ExecutionMode = ""
    ExecutionModeSequential ExecutionMode = "sequential"
    ExecutionModeParallel   ExecutionMode = "parallel"
)
```

For `Parameters()`, you have two reasonable options in Go:

1. **Reflection-based** (the agent walks a Go struct type) — like `github.com/invopop/jsonschema`. Familiar TS-flavor.
2. **Builder-based** — explicit schema construction. More verbose, more control.

Pi uses typebox (option 1, but TypeScript-flavored). Either is fine for Go; reflection is closer to pi's ergonomics.

## What's elegant

- **`details` separated from `content`** — UI gets structured data, model gets text. No accidental leaks of raw structured data into the prompt.
- **Per-tool `executionMode` override** — most tools can run in parallel, but some (e.g., a tool that mutates shared state) can opt out individually.
- **Three-phase execution** with hooks at each phase — `beforeToolCall` blocks before doing work; `afterToolCall` patches after the fact. Predictable composition.
- **Error-as-value** discipline — the loop never has to handle a tool throwing.

## TS-specific (skip in Go)

- `prepareArguments` shim for tools is a band-aid for raw-string-args from older models. **Skip if your tool args are always parsed JSON.**
- typebox-specific validation — use any JSON-schema validator in Go.