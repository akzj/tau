package provider

import "sync"

// ModelInfo describes a registered model.
type ModelInfo struct {
	ID        string   // "gpt-4o", "claude-sonnet-4-6"
	Name      string   // "GPT-4o", "Claude Sonnet 4"
	Provider  string   // "openai", "anthropic"
	MaxTokens int
	Features  []string // "streaming", "tools", "vision"
}

// ModelRegistry is a thread-safe model catalog.
type ModelRegistry struct {
	mu     sync.RWMutex
	models map[string]ModelInfo // id → info
	byProv map[string][]string  // provider → model ids
}

// NewModelRegistry creates a registry with known models pre-registered.
func NewModelRegistry() *ModelRegistry {
	r := &ModelRegistry{
		models: make(map[string]ModelInfo),
		byProv: make(map[string][]string),
	}
	// OpenAI models
	r.Register(ModelInfo{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", MaxTokens: 128000, Features: []string{"streaming", "tools", "vision"}})
	r.Register(ModelInfo{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", MaxTokens: 128000, Features: []string{"streaming", "tools"}})
	r.Register(ModelInfo{ID: "gpt-5.4", Name: "GPT-5.4", Provider: "openai", MaxTokens: 128000, Features: []string{"streaming", "tools"}})
	r.Register(ModelInfo{ID: "gpt-4", Name: "GPT-4", Provider: "openai", MaxTokens: 8192, Features: []string{"streaming", "tools"}})
	r.Register(ModelInfo{ID: "o1", Name: "O1", Provider: "openai", MaxTokens: 200000, Features: []string{"streaming", "tools"}})
	r.Register(ModelInfo{ID: "o1-mini", Name: "O1 Mini", Provider: "openai", MaxTokens: 100000, Features: []string{"streaming", "tools"}})
	// Anthropic models
	r.Register(ModelInfo{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4", Provider: "anthropic", MaxTokens: 200000, Features: []string{"streaming", "tools"}})
	r.Register(ModelInfo{ID: "claude-haiku-3-5", Name: "Claude Haiku 3.5", Provider: "anthropic", MaxTokens: 200000, Features: []string{"streaming", "tools"}})
	r.Register(ModelInfo{ID: "claude-opus-4", Name: "Claude Opus 4", Provider: "anthropic", MaxTokens: 200000, Features: []string{"streaming", "tools"}})
	return r
}

// Register adds a model to the registry.
func (r *ModelRegistry) Register(info ModelInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[info.ID] = info
	r.byProv[info.Provider] = append(r.byProv[info.Provider], info.ID)
}

// Lookup finds a model by ID.
func (r *ModelRegistry) Lookup(id string) (ModelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.models[id]
	return info, ok
}

// List returns all models for a provider (empty = all).
func (r *ModelRegistry) List(providerID string) []ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ModelInfo
	if providerID == "" {
		for _, info := range r.models {
			result = append(result, info)
		}
	} else {
		for _, id := range r.byProv[providerID] {
			if info, ok := r.models[id]; ok {
				result = append(result, info)
			}
		}
	}
	return result
}
