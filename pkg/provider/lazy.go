package provider

import (
	"fmt"
	"sync"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
	"github.com/akzj/tau/providers/anthropic-messages"
	"github.com/akzj/tau/providers/google-genai"
	"github.com/akzj/tau/providers/openai-completions"
)

// ProviderLoader lazy-loads and caches providers.
type ProviderLoader struct {
	mu        sync.Mutex
	factories map[string]func() (core.Provider, error)
	cache     map[string]core.Provider
	once      map[string]*sync.Once
}

// NewProviderLoader creates a loader with built-in factories.
func NewProviderLoader() *ProviderLoader {
	l := &ProviderLoader{
		factories: make(map[string]func() (core.Provider, error)),
		cache:     make(map[string]core.Provider),
		once:      make(map[string]*sync.Once),
	}
	// Built-in factories
	l.RegisterFactory("openai", func() (core.Provider, error) {
		return openai_completions.NewOpenAICompletionsProvider()
	})
	l.RegisterFactory("anthropic", func() (core.Provider, error) {
		return anthropic_messages.NewAnthropicMessagesProvider()
	})
	l.RegisterFactory("google", func() (core.Provider, error) {
		return google_genai.NewProvider()
	})
	l.RegisterFactory("faux", func() (core.Provider, error) {
		return faux.New(), nil
	})
	return l
}

// RegisterFactory adds a provider factory.
func (l *ProviderLoader) RegisterFactory(id string, factory func() (core.Provider, error)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.factories[id] = factory
	l.once[id] = &sync.Once{}
}

// Load returns a cached provider or creates one via factory.
func (l *ProviderLoader) Load(id string) (core.Provider, error) {
	l.mu.Lock()
	factory, ok := l.factories[id]
	if !ok {
		l.mu.Unlock()
		return nil, fmt.Errorf("unknown provider: %s (available: %v)", id, l.availableLocked())
	}
	once := l.once[id]
	l.mu.Unlock()

	var prov core.Provider
	var err error
	once.Do(func() {
		prov, err = factory()
		if err == nil {
			l.mu.Lock()
			l.cache[id] = prov
			l.mu.Unlock()
		}
	})

	l.mu.Lock()
	defer l.mu.Unlock()
	if p, ok := l.cache[id]; ok {
		return p, nil
	}
	return nil, err
}

func (l *ProviderLoader) availableLocked() []string {
	var ids []string
	for id := range l.factories {
		ids = append(ids, id)
	}
	return ids
}
