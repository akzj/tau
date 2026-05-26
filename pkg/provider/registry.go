package provider

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed models.json
var modelsJSON embed.FS

// ModelCost describes per-token pricing.
type ModelCost struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

// ModelInfo describes a registered model.
type ModelInfo struct {
	ID            string    `json:"id"`            // "gpt-4o", "claude-sonnet-4-6"
	Name          string    `json:"name"`          // "GPT-4o", "Claude Sonnet 4"
	Provider      string    `json:"provider"`      // "openai", "anthropic"
	ContextWindow int       `json:"contextWindow"` // max context tokens
	MaxTokens     int       `json:"maxTokens"`     // max output tokens
	InputTypes    []string  `json:"inputTypes"`    // "text", "image", "tool_use"
	Cost          ModelCost `json:"cost"`          // per-million-token pricing
	Features      []string  `json:"features,omitempty"`
}

// ModelRegistry is a thread-safe model catalog.
type ModelRegistry struct {
	mu     sync.RWMutex
	models map[string]ModelInfo // id → info
	byProv map[string][]string  // provider → model ids
}

// LoadModelRegistry loads models from the embedded models.json.
func LoadModelRegistry() (*ModelRegistry, error) {
	data, err := modelsJSON.ReadFile("models.json")
	if err != nil {
		return nil, fmt.Errorf("read models.json: %w", err)
	}
	var wrapper struct {
		Models []ModelInfo `json:"models"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse models.json: %w", err)
	}
	r := &ModelRegistry{
		models: make(map[string]ModelInfo),
		byProv: make(map[string][]string),
	}
	for _, info := range wrapper.Models {
		info.Features = []string{"streaming", "tools"}
		r.Register(info)
	}
	return r, nil
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