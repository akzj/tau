# 02 — Provider Abstraction

> **Scope**: The provider interface, wire/vendor double-layer, and Compat sealed system.
> **Basis**: `04-design-principles.md` (R1 sealed types, R4 Session-scoped registries, R5 zero product knowledge).
> **Reads with**: `01-core-design.md` (Session owns ProviderRegistry; Loop consumes raw ProviderEvent).

---

## §1 — Provider Interface

```go
type Provider interface {
    Stream(ctx context.Context, req StreamRequest) (<-chan ProviderEvent, error)
    Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
}
```

- **`Stream` is the primary path** — all LLM interactions go through streaming. Non-streaming consumers call `Stream` + drain channel; the `Complete` convenience path mirrors this.
- **`Complete` is a separate entry** for single-shot calls: compaction, summarization, branching. These bypass the Loop entirely.
- **Function-shaped, not object-shaped** (C10). No lifecycle (`Open`/`Close`), no stateful client, no `init()`. A provider is a pure function from request to event channel.
- **Channel not iterator** (W3-lite decision): Go channels are the only native multi-source fusion primitive. Loop needs to `select` across provider, tool completion, and cancellation simultaneously.

---

## §2 — WireAPI & ModelSpec

```go
type WireAPI string
const (
    WireOpenAICompletions WireAPI = "openai-completions"
    WireOpenAIResponses          = "openai-responses"
    WireAnthropicMessages        = "anthropic-messages"
    WireGoogleGenerativeAI       = "google-generative-ai"
    WireGoogleVertex             = "google-vertex"
    WireBedrockConverse          = "bedrock-converse"
    WireMistralConversations     = "mistral-conversations"
    WireAzureOpenAIResponses     = "azure-openai-responses"
    WireOpenAICodexResponses     = "openai-codex-responses"
)

type ModelSpec struct {
    Name    string
    WireAPI WireAPI   // dispatch key → internal wire implementation table
}
```

**The central insight (from pi production data)**: ~30 vendors map to only 9 wire protocols.
- **Vendor** = business entity (who bills, what auth, which models).
- **Wire protocol** = HTTP contract (how to call, what JSON shape).

Pi confused these: 30 vendors → one global `apiProviderRegistry` keyed by API kind, with compat flags doing vendor-specific tuning. Tau separates them at the registration layer.

**Adding a vendor**: 1 `RegisterVendor` call. Does not require changing `core/`.
**Adding a wire protocol**: rare — requires new `WireAPI` constant + new `WireCompat` concrete type + new wire implementation. Requires a core-package release.

---

## §3 — Request & Response Types

```go
type StreamRequest struct {
    Model             ModelSpec
    Messages          []Message
    Tools             []ToolSpec
    Options           ProviderOptions
    TransformToolName func(string) string          // C7: caller-injected, core knows zero tool names
    OnPayload         func(payload any) (any, error) // pre-marshal hook (pi types.ts:109)
    OnResponse        func(resp HTTPResponse)        // post-response inspection (pi types.ts:114)
}

type ProviderEvent struct {
    // ... delta/content fields ...
    Usage *Usage   // partial or nil for mid-stream events
}

type Usage struct {
    Input, Output, CacheRead, CacheWrite int
}

type CompleteRequest struct {
    Model    ModelSpec
    Messages []Message
    Options  ProviderOptions
}

type CompleteResponse struct {
    Content []Content
    Usage   Usage
}
```

**Design notes**:
- **`TransformToolName`** is caller-injected (C7). Core never knows "bash", "read", "write" — the product layer provides the mapping.
- **`OnPayload`/`OnResponse`** are callbacks on `StreamRequest`, not hooks on `HookSet`. They follow the same pattern as `TransformToolName` — injected per-request, not registered globally (pi `types.ts:109/114` evidence).
- **`Usage` tracks cache separately**: `CacheRead`/`CacheWrite` are 10× cheaper than `Input`/`Output`. Cost calculation treats all four independently.

---

## §4 — Compat Sealed Interface

```go
type WireCompat interface {
    wireCompat() // sealed marker — no external implementations
}

type OpenAICompletionsCompat struct { /* apiKey, org, 20+ flags */ }
type AnthropicMessagesCompat   struct { /* apiKey, authToken, ... */ }
type OpenAIResponsesCompat     struct { /* apiKey, ... */ }
```

- **Sealed, not `any`** (R1). Three concrete types, one per wire protocol family. Compiler rejects `any` assignment.
- **Accepts R1 compile→runtime downgrade**: The `WireAPI` ↔ `Compat` type alignment is checked at `RegisterVendor` time via runtime invariant (not generics). If misaligned → `panic` (P2 panic-fast, init-phase config bug).
- **Why not generics `Compat[A WireAPI]`?** Generics would give compile-time guarantee but infect `VendorConfig` with type parameters and cascade to `RegisterVendor`. The cognitive cost outweighs the marginal safety gain for an init-time check.
- **20+ flags will accrue** (pi lesson). Each wire protocol carries vendor-specific knobs: `supportsReasoningEffort`, `thinkingFormat`, `cacheControlFormat`, `requiresToolResultName`, etc. Plan for growth — the struct is the right home.

---

## §5 — ProviderRegistry (Form B)

```go
type VendorConfig struct {
    API     WireAPI
    BaseURL string
    OAuth   OAuthSource          // per-call token refresh (pi getApiKey pattern)
    Models  []ModelSpec
    Routing VendorTypedRouting   // vendor-typed, NOT embedded in Compat
    Compat  WireCompat           // init-time invariant: must match cfg.API
}

type VendorTypedRouting interface {
    vendorRouting() // sealed marker
}

type ProviderRegistry struct{ /* ... */ }
func (r *ProviderRegistry) RegisterVendor(name string, cfg VendorConfig) error
func (r *ProviderRegistry) Get(name string) (Provider, bool)
```

**Registration flow**:
```
RegisterVendor("my-openrouter", VendorConfig{
    API:     WireOpenAICompletions,          // dispatch key
    BaseURL: "https://openrouter.ai",
    OAuth:   openrouterTokenSource,
    Compat:  OpenAICompletionsCompat{...},   // invariant-checked against API
    Routing: OpenRouterRouting{...},         // vendor-specific, not in Compat
})
  → internal wire table lookup (9 entries, init-registered)
    → assertCompatMatchesAPI(cfg.API, cfg.Compat)  // mismatch → panic (P2)
      → create bound Provider
```

- **`ProviderRegistry` is a Session field** (R4), not a package-level `var`. No global mutable state.
- **`OAuth` is per-call** (pi surprise #8). Short-lived tokens (GitHub Copilot, OpenAI Codex) can expire mid-tool. Provider implementation must re-resolve token on each `Stream`/`Complete` call, not cache at registration time.
- **`Routing` is vendor-typed, not in Compat** (A2 Q4). Pi's `OpenRouterRouting` embedded inside `OpenAICompletionsCompat` was a lowest-common-denominator anti-pattern.

---

## §6 — Double-Layer Architecture

```
Product Layer (coding-agent, webui)
  └─ RegisterVendor("name", VendorConfig{API, BaseURL, OAuth, Models, Compat, Routing})
       └─ ProviderRegistry (Session-scoped, per-user isolation)
            └─ Internal wire impl table (9 entries, package-init registered)
                 └─ Wire impl → HTTP → LLM
```

**Key separation**:
- **User surface**: vendor granularity — name, auth, model list, routing. `RegisterVendor` is the only API product code needs.
- **Internal surface**: wire protocol granularity — 9 `WireAPI` constants, registered at `init()` time. Product code never touches these.
- **Dispatch**: `ModelSpec.WireAPI` → wire impl table → concrete `Provider`. Transparent to product layer.

---

## §7 — W3-Lite Cross-System Note

Three Go SDK libraries surveyed (langchaingo, go-openai, openai-go) all use iterator/callback patterns — not channels. Tau's choice of `<-chan ProviderEvent` is a **conscious harness-layer trade-off**:

- **Channel = Go's only native multi-source fusion primitive**. Loop must `select` across provider stream, tool completions, and cancellation in one goroutine. Iterators don't fuse.
- **Cost**: channels require a producer goroutine per call. For agent workloads (≤10 concurrent LLM calls per session), this is negligible.
- **If this becomes a bottleneck** (Phase 5+, ≥100 concurrent calls): migrate to `iter.Seq[ProviderEvent]` with a fusion adapter. The `Provider` interface signature change is localized.

---

## §8 — Connection Points

| This file → | Why |
|-------------|-----|
| `01-core-design.md` | `ProviderRegistry` on Session; Loop consumes raw `ProviderEvent`. |
| `04-design-principles.md` | R1 (Compat sealed, no `any`), R4 (registry scoped to Session), R5 (zero product names in core). |
| `05-access-points.md` | `RegisterVendor` is the primary access point; `Complete` for compaction. |
| `06-compaction.md` | Compaction uses `Provider.Complete` for summarization. |
| `07-go-structure.md` | `providers/{wire-name}/` subdirectories per wire protocol. |

---

*02-provider.md — the wire knows, the vendor pays, the core is indifferent.*
