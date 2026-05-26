# 4. Provider Abstraction

Source: `packages/ai/src/types.ts` (575 LOC), `packages/ai/src/api-registry.ts` (98 LOC), `packages/ai/src/stream.ts` (59 LOC), `packages/ai/src/utils/event-stream.ts` (88 LOC), `packages/ai/src/providers/register-builtins.ts` (406 LOC), `packages/ai/src/models.ts`.

## 4.1 Unified message types

```
TextContent | ThinkingContent | ImageContent | ToolCall   (assistant content blocks)

UserMessage         { role:"user", content: string | (TextContent|ImageContent)[], timestamp }
AssistantMessage    { role:"assistant", content: (TextContent|ThinkingContent|ToolCall)[],
                      api, provider, model, usage, stopReason, errorMessage?, timestamp }
ToolResultMessage   { role:"toolResult", toolCallId, toolName, content, details?, isError, timestamp }

Context             = { systemPrompt?, messages: Message[], tools?: Tool[] }
StopReason          = "stop" | "length" | "toolUse" | "error" | "aborted"
```

Note: `ThinkingContent` may carry an opaque `thinkingSignature` (used by OpenAI Responses for the reasoning item ID) and a `redacted` flag (Anthropic's redacted-thinking case where the encrypted payload must round-trip via the signature for multi-turn continuity).

`ToolCall` may carry a Google-specific `thoughtSignature` for reusing thought context.

## 4.2 Streaming protocol unification

Every provider streams a sequence of typed events ending in either `done` or `error`:

```
{type:"start", partial}
{type:"text_start"|"text_delta"|"text_end", contentIndex, ...}
{type:"thinking_start"|"thinking_delta"|"thinking_end", contentIndex, ...}
{type:"toolcall_start"|"toolcall_delta"|"toolcall_end", contentIndex, ...}
{type:"done", reason, message}        // success
{type:"error", reason: "error"|"aborted", error: AssistantMessage}
```

Every event carries the **full current partial assistant message** (`partial`), so consumers don't need to maintain replay state. This is intentional — it simplifies UI rendering at the cost of per-event cloning.

`AssistantMessageEventStream` (in `utils/event-stream.ts`) is a tiny `AsyncIterable` queue (~88 LOC) with `push`, `end`, `result()`. Consumers `for await` events; the harness only needs `result()`.

## 4.3 Provider plugin shape

```typescript
interface ApiProvider<TApi, TOptions extends StreamOptions> {
    api: TApi;
    stream:       (model, context, options?) => AssistantMessageEventStream;
    streamSimple: (model, context, options?) => AssistantMessageEventStream;
}
registerApiProvider({api, stream, streamSimple}, sourceId?);
```

The registry is a global `Map<api, provider>`, **keyed by API kind** (e.g., `"openai-completions"`, `"anthropic-messages"`, `"google-generative-ai"`), **not by provider company**.

### The central insight: wire protocol, not vendor

**The unit of pluggability is the wire protocol, not the company.** OpenAI Completions, Anthropic Messages, OpenAI Responses, Google GenAI, Bedrock Converse, Mistral Conversations are each one provider; all the OpenAI-compatible vendors share the `openai-completions` plugin and differ via per-`Model` `compat` flags.

### `KnownProvider` (vendor list, ~30 entries)

```
amazon-bedrock | anthropic | google | google-vertex | openai
azure-openai-responses | openai-codex | deepseek | github-copilot
xai | groq | cerebras | openrouter | vercel-ai-gateway | zai
mistral | minimax | minimax-cn | moonshotai | moonshotai-cn
huggingface | fireworks | together | opencode | opencode-go
kimi-coding | cloudflare-workers-ai | cloudflare-ai-gateway
xiaomi | xiaomi-token-plan-cn | xiaomi-token-plan-ams | xiaomi-token-plan-sgp
```

Most of these are accessed through the `openai-completions` provider with different `Model.compat` settings.

### `KnownApi` (wire protocol list, only 9 entries)

```
openai-completions | mistral-conversations
openai-responses | azure-openai-responses | openai-codex-responses
anthropic-messages | bedrock-converse-stream
google-generative-ai | google-vertex
```

This is the dimension your Go port should structure around.

## 4.4 The `compat` field — plan for ugly knobs

`OpenAICompletionsCompat` carries **20+ flags**:

```typescript
interface OpenAICompletionsCompat {
    supportsStore?: boolean;
    supportsDeveloperRole?: boolean;
    supportsReasoningEffort?: boolean;
    supportsUsageInStreaming?: boolean;
    maxTokensField?: "max_completion_tokens" | "max_tokens";
    requiresToolResultName?: boolean;
    requiresAssistantAfterToolResult?: boolean;
    requiresThinkingAsText?: boolean;
    requiresReasoningContentOnAssistantMessages?: boolean;
    thinkingFormat?: "openai" | "openrouter" | "deepseek"
                   | "together" | "zai" | "qwen" | "qwen-chat-template";
    openRouterRouting?: OpenRouterRouting;     // 10+ sub-fields
    vercelGatewayRouting?: VercelGatewayRouting;
    zaiToolStream?: boolean;
    supportsStrictMode?: boolean;
    cacheControlFormat?: "anthropic";
    sendSessionAffinityHeaders?: boolean;
    supportsLongCacheRetention?: boolean;
}
```

Pi has clearly hit every weird API in production:
- DeepSeek: `thinking: { type }`
- Together: `reasoning: { enabled }`
- ZAI: `enable_thinking` (top-level)
- Qwen: `enable_thinking` (top-level OR inside `chat_template_kwargs`)
- Anthropic-on-Fireworks: `cache_control` skipped on tools
- Fireworks: needs `x-session-affinity` header for prompt cache routing

**Plan for ugly compat flags — they will accrue as you onboard real-world endpoints.**

`AnthropicMessagesCompat` and `OpenAIResponsesCompat` have similar (smaller) flag sets.

## 4.5 Lazy provider loading

Built-ins are **lazily loaded** via dynamic import. `streamAnthropic` is a lazy stub registered at module load; the actual `./anthropic.ts` (1212 LOC, depends on `@anthropic-ai/sdk`) is only imported when a request is made.

This keeps cold-start small. **In Go**, you can blank-import all providers at init and rely on the linker to dead-strip unused ones, or use build tags to exclude expensive providers (e.g., the AWS SDK for Bedrock).

## 4.6 Model registry

`MODELS` is a generated table in `models.generated.ts` (16115 LOC, do not edit by hand — regenerated via `scripts/generate-models.ts`).

```typescript
getModel(provider, modelId) => Model<api>
```

```typescript
interface Model<TApi> {
    id: string;
    name: string;
    api: TApi;
    provider: Provider;
    baseUrl: string;
    reasoning: boolean;
    thinkingLevelMap?: ThinkingLevelMap;  // pi level → provider value
    input: ("text" | "image")[];
    cost: { input, output, cacheRead, cacheWrite };  // $/M tokens
    contextWindow: number;
    maxTokens: number;
    headers?: Record<string, string>;
    compat?: ...;  // type-narrowed by api
}
```

Pre-known model metadata avoids per-request "what context window does this support" guessing.

**For the Go port**: ship a swappable JSON file (or fetch dynamically from a model index endpoint). Embedding 16K LOC of generated tables in a Go module is a maintenance pain.

## 4.7 Token counting / budget

Tokens are **provider-reported** (`Usage{input, output, cacheRead, cacheWrite, totalTokens, cost{...}}`) — pi does NOT run a tokenizer.

`calculateCost(model, usage)` multiplies by `model.cost.*`. The compaction estimator (`chars/4` with image penalty) is a fallback for messages without provider usage.

**Important**: `Usage` separates `cacheRead` / `cacheWrite` from `input` / `output` because prompt-cache hits are 10× cheaper than fresh input tokens. Cost calculation treats all four separately.

## 4.8 Error model

Provider errors are **NOT thrown** out of `stream()`. They are emitted as the **final `error` event** carrying an `AssistantMessage` with `stopReason: "error"|"aborted"` and `errorMessage`.

Retries are pushed into the underlying SDK (`maxRetries` option to OpenAI/Anthropic SDK clients, default 2). `maxRetryDelayMs` caps server-requested backoff so very long retries fail fast for higher-level retry logic. Auth failures are just normal errors.

`getApiKey` is **per-call, not per-session**. Designed for short-lived OAuth tokens (GitHub Copilot, OpenAI Codex). Tools can run for minutes; the token at agent-start may expire before the next provider call. Always re-resolve.

## Go sketch

```go
type Provider interface {
    API() string
    Stream(ctx context.Context, model Model, requestCtx Context, opts StreamOptions) (<-chan StreamEvent, error)
}

type StreamEvent interface { isStreamEvent() }

type StartEvent       struct { Partial AssistantMessage }
type TextDeltaEvent   struct { ContentIndex int; Delta string; Partial AssistantMessage }
type DoneEvent        struct { Reason StopReason; Message AssistantMessage }
type ErrorEvent       struct { Reason StopReason; Error AssistantMessage }
// ... etc

type ProviderRegistry struct {
    providers map[string]Provider  // keyed by API kind
}

type Model struct {
    ID, Name, API, Provider, BaseURL string
    ContextWindow, MaxTokens         int
    Cost                              CostPerMToken
    Compat                            any  // type-asserted by provider
}
```