package provider

import (
	"testing"
)

func TestLoadModelRegistryEmbeds(t *testing.T) {
	r, err := LoadModelRegistry()
	if err != nil {
		t.Fatalf("LoadModelRegistry: %v", err)
	}
	models := r.List("")
	if len(models) == 0 {
		t.Error("expected models from embedded models.json")
	}
}

func TestModelRegistryLookup(t *testing.T) {
	r, err := LoadModelRegistry()
	if err != nil {
		t.Fatal(err)
	}
	info, ok := r.Lookup("gpt-5.4")
	if !ok {
		t.Fatal("gpt-5.4 not found")
	}
	if info.Provider != "openai" {
		t.Errorf("expected openai, got %s", info.Provider)
	}
	if info.ContextWindow == 0 {
		t.Error("context window should not be 0")
	}
	if info.Cost.Input == 0 {
		t.Error("cost input should not be 0")
	}
	if info.Features == nil {
		t.Error("expected features to be set")
	}
}

func TestModelRegistryListByProvider(t *testing.T) {
	r, _ := LoadModelRegistry()
	models := r.List("anthropic")
	if len(models) == 0 {
		t.Error("expected anthropic models")
	}
	for _, m := range models {
		if m.Provider != "anthropic" {
			t.Errorf("expected anthropic, got %s", m.Provider)
		}
	}
}

func TestModelRegistryDuplicate(t *testing.T) {
	r, _ := LoadModelRegistry()
	// Register same model again — should overwrite
	r.Register(ModelInfo{ID: "gpt-5.4", Name: "duplicate", Provider: "openai"})
	info, _ := r.Lookup("gpt-5.4")
	if info.Name != "duplicate" {
		t.Errorf("expected overwrite to 'duplicate', got %q", info.Name)
	}
}

func TestModelRegistryLookupMissing(t *testing.T) {
	r, _ := LoadModelRegistry()
	_, ok := r.Lookup("nonexistent-model-id")
	if ok {
		t.Error("expected false for missing model")
	}
}

func TestModelRegistryListAll(t *testing.T) {
	r, _ := LoadModelRegistry()
	models := r.List("")
	providers := map[string]bool{}
	for _, m := range models {
		providers[m.Provider] = true
	}
	if !providers["openai"] || !providers["anthropic"] || !providers["google"] {
		t.Errorf("expected all providers in full list, got: %v", providers)
	}
}

func TestModelRegistryListEmptyProvider(t *testing.T) {
	r, _ := LoadModelRegistry()
	models := r.List("nonexistent-provider")
	if len(models) != 0 {
		t.Errorf("expected 0 models for unknown provider, got %d", len(models))
	}
}

func TestNewProviderLoader(t *testing.T) {
	l := NewProviderLoader()
	if l == nil {
		t.Fatal("expected non-nil ProviderLoader")
	}
	// Faux provider is registered by default
	prov, err := l.Load("faux")
	if err != nil {
		t.Fatalf("Load faux: %v", err)
	}
	if prov == nil {
		t.Error("expected non-nil faux provider")
	}
}

func TestProviderLoaderCached(t *testing.T) {
	l := NewProviderLoader()
	prov1, err := l.Load("faux")
	if err != nil {
		t.Fatal(err)
	}
	prov2, err := l.Load("faux")
	if err != nil {
		t.Fatal(err)
	}
	if prov1 != prov2 {
		t.Error("expected same cached instance")
	}
}

func TestProviderLoaderUnknown(t *testing.T) {
	l := NewProviderLoader()
	_, err := l.Load("nonexistent-provider")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestModelInfoFields(t *testing.T) {
	info := ModelInfo{
		ID:            "test-id",
		Name:          "Test Model",
		Provider:      "openai",
		ContextWindow: 128000,
		MaxTokens:     16384,
		InputTypes:    []string{"text", "image"},
		Cost:          ModelCost{Input: 2.5, Output: 10},
		Features:      []string{"streaming"},
	}
	if info.ID != "test-id" {
		t.Error("ID field mismatch")
	}
	if info.Name != "Test Model" {
		t.Error("Name field mismatch")
	}
	if info.ContextWindow != 128000 {
		t.Error("ContextWindow field mismatch")
	}
	if info.Cost.Input != 2.5 || info.Cost.Output != 10 {
		t.Error("Cost fields mismatch")
	}
}

func TestModelCostFields(t *testing.T) {
	mc := ModelCost{Input: 1.5, Output: 6.0}
	if mc.Input != 1.5 {
		t.Error("ModelCost.Input mismatch")
	}
	if mc.Output != 6.0 {
		t.Error("ModelCost.Output mismatch")
	}
}
