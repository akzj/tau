package core

import "context"

// WireAPI is a sealed string type for wire protocol dispatch.
type WireAPI string

// Known wire protocols.
const (
	WireOpenAICompletions    WireAPI = "openai-completions"
	WireOpenAIResponses      WireAPI = "openai-responses"
	WireAnthropicMessages    WireAPI = "anthropic-messages"
	WireGoogleGenerativeAI   WireAPI = "google-generative-ai"
	WireGoogleVertex         WireAPI = "google-vertex"
	WireBedrockConverse      WireAPI = "bedrock-converse"
	WireMistralConversations WireAPI = "mistral-conversations"
	WireAzureOpenAIResponses WireAPI = "azure-openai-responses"
	WireOpenAICodexResponses WireAPI = "openai-codex-responses"
)

// ModelSpec identifies a model and its wire protocol.
type ModelSpec struct {
	Name string
	API  WireAPI
}

// ProviderOptions carries optional provider parameters.
type ProviderOptions struct {
	Temperature float64
	MaxTokens   int
	TopP        float64
	Stop        []string
}

// ToolSpec is a tool schema sent to the LLM (not the executor).
type ToolSpec struct {
	Name        string
	Description string
	Schema      any // JSON Schema value — will be marshaled
}

// StreamRequest carries everything a Provider needs for streaming.
type StreamRequest struct {
	Model        ModelSpec
	Messages     []Message
	Tools        []ToolSpec
	Options      ProviderOptions
	SystemPrompt string
	// TransformToolName optionally renames tools before sending to the LLM.
	TransformToolName func(string) string
	// OnPayload is a per-request payload transform hook.
	OnPayload func(payload any) (any, error)
	// OnResponse is a per-request response hook.
	OnResponse func(statusCode int, headers map[string][]string)
}

// CompleteRequest for non-streaming completion.
type CompleteRequest struct {
	Model        ModelSpec
	Messages     []Message
	SystemPrompt string
	Options      ProviderOptions
}

// CompleteResponse from non-streaming completion.
type CompleteResponse struct {
	Content string
	Usage   Usage
}

// Usage tracks token usage.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Provider is a pure function: (ctx, req) → event stream.
// It is NOT a stateful client object.
type Provider interface {
	Stream(ctx context.Context, req StreamRequest) (<-chan ProviderEvent, error)
	Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
}

// OAuthSource provides tokens for OAuth-authenticated providers.
type OAuthSource interface {
	Token(ctx context.Context) (string, error)
}

// WireCompat is a sealed interface for wire-specific compatibility transforms.
type WireCompat interface {
	wireCompat() // sealed marker
}

// OpenAICompletionsCompat holds wire-specific flags for the OpenAI Chat Completions API.
type OpenAICompletionsCompat struct {
	SupportsReasoningEffort bool // supports reasoning_effort param
	SupportsStrictMode      bool // supports strict mode for function calling
	SupportsStore           bool // supports store param
	MaxTokensField          bool // uses max_tokens (vs max_completion_tokens)
	ResponseFormatField     bool // supports response_format
	TemperatureField        bool // supports temperature (default true)
	TopPField               bool // supports top_p
	FrequencyPenaltyField   bool
	PresencePenaltyField    bool
	SupportsStop            bool
	SupportsN               bool
	SupportsLogprobs        bool
	SupportsStreamOptions   bool
}

func (OpenAICompletionsCompat) wireCompat() {}

// AnthropicMessagesCompat holds wire-specific flags for the Anthropic Messages API.
type AnthropicMessagesCompat struct {
	ThinkingFormat        bool // supports thinking (enabled/disabled)
	SupportsCacheControl  bool // supports ephemeral cache_control
	MaxTokensField        bool // uses max_tokens (always true for Anthropic)
	SupportsStopSequences bool
	SupportsTopK          bool
	TemperatureField      bool
	SupportsToolChoice    bool
}

func (AnthropicMessagesCompat) wireCompat() {}

// OpenAIResponsesCompat holds wire-specific flags for the OpenAI Responses API.
type OpenAIResponsesCompat struct {
	SupportsThinking           bool // supports thinking via reasoning.effort
	ReasoningEffortField       bool
	SupportsStore              bool
	SupportsPreviousResponseID bool
	SupportsInstructions       bool
	SupportsParallelToolCalls  bool
	SupportsWebSearch          bool
	SupportsTruncation         bool
}

func (OpenAIResponsesCompat) wireCompat() {}

// VendorTypedRouting is a sealed interface for vendor-specific routing logic.
type VendorTypedRouting interface {
	vendorRouting() // sealed marker
}

// VendorConfig registers a vendor with its wire protocol.
type VendorConfig struct {
	API     WireAPI
	BaseURL string
	OAuth   OAuthSource         // nil for API-key auth
	Models  []ModelSpec
	Routing VendorTypedRouting  // vendor-specific routing
	Compat  WireCompat          // wire-specific compatibility transforms
}

// ProviderRegistry is scoped to a Session (NOT package-level).
type ProviderRegistry struct {
	vendors   map[string]VendorConfig // name → config
	defaults  map[WireAPI]string      // wire → default vendor name
	providers map[string]Provider     // name → Provider instance
}

// NewProviderRegistry creates an empty ProviderRegistry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		vendors:   make(map[string]VendorConfig),
		defaults:  make(map[WireAPI]string),
		providers: make(map[string]Provider),
	}
}

// RegisterProvider registers a pre-constructed Provider by name.
// This is the simple path for demo; RegisterVendor+wire dispatch is Phase 5+.
func (r *ProviderRegistry) RegisterProvider(name string, p Provider) {
	r.providers[name] = p
}

// Get returns a Provider by name.
func (r *ProviderRegistry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// RegisterVendor registers a vendor. Single user-facing entry point (form B).
// The first vendor registered for a wire protocol becomes its default.
func (r *ProviderRegistry) RegisterVendor(name string, cfg VendorConfig) error {
	if name == "" {
		return &TauError{Code: ErrSession, Message: "vendor name must not be empty"}
	}
	if cfg.API == "" {
		return &TauError{Code: ErrSession, Message: "vendor must specify an API"}
	}
	r.vendors[name] = cfg
	if _, exists := r.defaults[cfg.API]; !exists {
		r.defaults[cfg.API] = name
	}
	return nil
}

// GetVendor returns the VendorConfig for a named vendor.
func (r *ProviderRegistry) GetVendor(name string) (VendorConfig, bool) {
	v, ok := r.vendors[name]
	return v, ok
}

// ResolveByModel finds the vendor for a given model spec.
func (r *ProviderRegistry) ResolveByModel(model ModelSpec) (VendorConfig, bool) {
	if defaultName, ok := r.defaults[model.API]; ok {
		return r.vendors[defaultName], true
	}
	return VendorConfig{}, false
}