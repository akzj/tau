package core

import (
	"fmt"
	"strings"
	"sync"
)

// SemanticFact represents an extracted piece of knowledge.
type SemanticFact struct {
	ID              string   `json:"id"`
	Content         string   `json:"content"`
	SourceEpisode   string   `json:"source_episode"`
	Confidence      float64  `json:"confidence"` // 0.0-1.0
	Tags            []string `json:"tags"`
	TimesReferenced int      `json:"times_referenced"`
}

// SemanticMemory stores extracted knowledge facts.
type SemanticMemory struct {
	mu    sync.RWMutex
	facts []SemanticFact
	store StorageBackend // nil = in-memory only
}

// NewSemanticMemory creates a semantic memory store.
func NewSemanticMemory(store StorageBackend) *SemanticMemory {
	sm := &SemanticMemory{store: store}
	if store != nil {
		sm.load()
	}
	return sm
}

// Add stores a new semantic fact.
func (sm *SemanticMemory) Add(fact SemanticFact) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	fact.ID = fmt.Sprintf("sem-%d", len(sm.facts)+1)
	sm.facts = append(sm.facts, fact)
	if sm.store != nil {
		sm.store.Write("sem:"+fact.ID, []byte(fact.Content+"|"+strings.Join(fact.Tags, ",")))
	}
}

// Query finds facts matching the given keywords/tags.
func (sm *SemanticMemory) Query(query string, limit int) []SemanticFact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if limit <= 0 {
		limit = 5
	}
	q := strings.ToLower(query)
	var results []SemanticFact
	for i := len(sm.facts) - 1; i >= 0; i-- {
		f := sm.facts[i]
		if strings.Contains(strings.ToLower(f.Content), q) || hasMatchingTag(f.Tags, q) {
			results = append(results, f)
			if len(results) >= limit {
				break
			}
		}
	}
	return results
}

// List returns all facts.
func (sm *SemanticMemory) List() []SemanticFact {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return append([]SemanticFact{}, sm.facts...)
}

// Prune removes facts below the given confidence threshold.
func (sm *SemanticMemory) Prune(minConfidence float64) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	var kept []SemanticFact
	removed := 0
	for _, f := range sm.facts {
		if f.Confidence >= minConfidence {
			kept = append(kept, f)
		} else {
			removed++
			if sm.store != nil {
				sm.store.Delete("sem:" + f.ID)
			}
		}
	}
	sm.facts = kept
	return removed
}

// Len returns the number of stored facts.
func (sm *SemanticMemory) Len() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.facts)
}

// ReplaceStore atomically replaces the storage backend and reloads.
func (sm *SemanticMemory) ReplaceStore(store StorageBackend) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.store = store
	if store != nil {
		// Reload: clear in-memory and reload from store
		sm.facts = nil
		sm.mu.Unlock()
		sm.loadWithLock()
		sm.mu.Lock()
	}
}

func (sm *SemanticMemory) load() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.loadWithLock()
}

func (sm *SemanticMemory) loadWithLock() {
	keys, err := sm.store.List("sem:")
	if err != nil {
		return
	}
	// Build a set of existing IDs to avoid duplicate reload
	existing := make(map[string]bool)
	for _, f := range sm.facts {
		existing[f.ID] = true
	}
	for _, k := range keys {
		// Strip the "sem:" prefix to get the local ID
		localID := strings.TrimPrefix(k, "sem:")
		if existing[localID] {
			continue
		}
		data, err := sm.store.Read(k)
		if err != nil {
			continue
		}
		parts := strings.SplitN(string(data), "|", 2)
		content := parts[0]
		var tags []string
		if len(parts) > 1 {
			tags = strings.Split(parts[1], ",")
		}
		sm.facts = append(sm.facts, SemanticFact{
			ID: localID, Content: content, Tags: tags, Confidence: 0.5,
		})
	}
}

func hasMatchingTag(tags []string, query string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}
