package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// --- Working Memory (short-term, volatile) ---

// Observation is a single working memory entry.
type Observation struct {
	Content    string    `json:"content"`
	Source     string    `json:"source"`
	Importance float64   `json:"importance"`
	Timestamp  time.Time `json:"timestamp"`
}

// WorkingMemory holds short-term observations for the current session.
type WorkingMemory struct {
	mu         sync.Mutex
	entries    []Observation
	maxEntries int
}

// NewWorkingMemory creates a working memory with a max entry limit.
func NewWorkingMemory(maxEntries int) *WorkingMemory {
	if maxEntries <= 0 {
		maxEntries = 100
	}
	return &WorkingMemory{maxEntries: maxEntries}
}

// Add adds an observation. Evicts least important if over capacity.
func (wm *WorkingMemory) Add(obs Observation) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	obs.Timestamp = time.Now()
	wm.entries = append(wm.entries, obs)
	if len(wm.entries) > wm.maxEntries {
		wm.evict()
	}
}

// Recall returns observations matching the query (simple substring match).
func (wm *WorkingMemory) Recall(query string, limit int) []Observation {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if limit <= 0 {
		limit = 10
	}
	var result []Observation
	for _, e := range wm.entries {
		if strings.Contains(strings.ToLower(e.Content), strings.ToLower(query)) {
			result = append(result, e)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// Summarize returns a compressed summary of recent observations.
func (wm *WorkingMemory) Summarize() string {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if len(wm.entries) == 0 {
		return "(empty)"
	}
	var lines []string
	recent := wm.entries
	if len(recent) > 20 {
		recent = recent[len(recent)-20:]
	}
	for _, e := range recent {
		lines = append(lines, fmt.Sprintf("[%s] %s", e.Source, e.Content))
	}
	return strings.Join(lines, "\n")
}

func (wm *WorkingMemory) evict() {
	minIdx := 0
	for i, e := range wm.entries {
		if e.Importance < wm.entries[minIdx].Importance {
			minIdx = i
		}
	}
	wm.entries = append(wm.entries[:minIdx], wm.entries[minIdx+1:]...)
}

// Stats returns source distribution of observations.
func (wm *WorkingMemory) Stats() map[string]int {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	sources := make(map[string]int)
	for _, e := range wm.entries {
		sources[e.Source]++
	}
	return sources
}

// Len returns the number of entries.
func (wm *WorkingMemory) Len() int {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return len(wm.entries)
}

// --- Episodic Memory (long-term, persisted) ---

// Episode records a completed action and its outcome.
type Episode struct {
	ID        string    `json:"id"`
	Trigger   string    `json:"trigger"`
	Action    string    `json:"action"`
	Outcome   string    `json:"outcome"`
	Lesson    string    `json:"lesson"`
	Tags      []string  `json:"tags"`
	Timestamp time.Time `json:"timestamp"`
}

// EpisodicMemory stores completed episodes.
type EpisodicMemory struct {
	mu          sync.Mutex
	episodes    []Episode
	dir         string
	maxEpisodes int
	store       StorageBackend // SQLite backend (nil = JSON fallback or in-memory)
	lru         *lruCache      // hot item cache for fast recall
}

// lruCache is a simple LRU cache for episodes.
type lruCache struct {
	items map[string]*Episode
	order []string
	max   int
}

func (c *lruCache) update(ep *Episode) {
	if c == nil {
		return
	}
	// Remove existing entry for this key
	for i, id := range c.order {
		if id == ep.ID {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	// Evict oldest if at capacity
	if len(c.order) >= c.max {
		oldest := c.order[0]
		delete(c.items, oldest)
		c.order = c.order[1:]
	}
	c.order = append(c.order, ep.ID)
	c.items[ep.ID] = ep
}

func (c *lruCache) get(id string) *Episode {
	if c == nil {
		return nil
	}
	return c.items[id]
}

// NewEpisodicMemory creates an episodic memory store.
// If store is non-nil, episodes are persisted to SQLite.
// If store is nil and dir is non-empty, episodes use JSON file persistence.
// If both are nil/empty, episodes are in-memory only.
func NewEpisodicMemory(dir string, maxEpisodes int, store StorageBackend) *EpisodicMemory {
	if maxEpisodes <= 0 {
		maxEpisodes = 1000
	}
	em := &EpisodicMemory{
		dir:         dir,
		maxEpisodes: maxEpisodes,
		store:       store,
		lru:         &lruCache{items: make(map[string]*Episode), max: 100, order: make([]string, 0, 100)},
	}
	// Load episodes from store or JSON
	if store != nil {
		eps, err := store.LoadEpisodes()
		if err == nil && len(eps) > 0 {
			em.episodes = eps
		}
	} else {
		em.load()
	}
	return em
}

// Record adds an episode.
func (em *EpisodicMemory) Record(ep Episode) {
	em.mu.Lock()
	defer em.mu.Unlock()
	ep.ID = fmt.Sprintf("ep-%d", len(em.episodes)+1)
	ep.Timestamp = time.Now()
	em.episodes = append(em.episodes, ep)
	if len(em.episodes) > em.maxEpisodes {
		em.episodes = em.episodes[1:]
	}
	// Persist
	if em.store != nil {
		em.store.SaveEpisode(ep)
	} else {
		em.save()
	}
	// Update LRU cache
	em.lru.update(&ep)
}

// Recall searches episodes by trigger/action/outcome/tags.
func (em *EpisodicMemory) Recall(query string, limit int) []Episode {
	em.mu.Lock()
	defer em.mu.Unlock()
	if limit <= 0 {
		limit = 10
	}
	q := strings.ToLower(query)
	var result []Episode
	for i := len(em.episodes) - 1; i >= 0; i-- {
		ep := em.episodes[i]
		if strings.Contains(strings.ToLower(ep.Trigger), q) ||
			strings.Contains(strings.ToLower(ep.Action), q) ||
			strings.Contains(strings.ToLower(ep.Outcome), q) ||
			containsTag(ep.Tags, q) {
			result = append(result, ep)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// ListByTag returns episodes matching a tag.
func (em *EpisodicMemory) ListByTag(tag string) []Episode {
	em.mu.Lock()
	defer em.mu.Unlock()
	var result []Episode
	for i := len(em.episodes) - 1; i >= 0; i-- {
		if containsTag(em.episodes[i].Tags, tag) {
			result = append(result, em.episodes[i])
		}
	}
	return result
}

// Forget removes episodes older than the given duration.
func (em *EpisodicMemory) Forget(olderThan time.Duration) int {
	em.mu.Lock()
	defer em.mu.Unlock()
	cutoff := time.Now().Add(-olderThan)
	kept := 0
	var newEps []Episode
	for _, ep := range em.episodes {
		if ep.Timestamp.After(cutoff) {
			newEps = append(newEps, ep)
			kept++
		}
	}
	removed := len(em.episodes) - kept
	em.episodes = newEps
	em.save()
	return removed
}

// Stats returns tag distribution.
func (em *EpisodicMemory) Stats() map[string]int {
	em.mu.Lock()
	defer em.mu.Unlock()
	tags := make(map[string]int)
	for _, ep := range em.episodes {
		for _, t := range ep.Tags {
			tags[t]++
		}
	}
	return tags
}

// Len returns the number of episodes.
func (em *EpisodicMemory) Len() int {
	em.mu.Lock()
	defer em.mu.Unlock()
	return len(em.episodes)
}

func (em *EpisodicMemory) save() {
	if em.dir == "" {
		return
	}
	os.MkdirAll(em.dir, 0755)
	path := filepath.Join(em.dir, "episodes.json")
	data, _ := json.MarshalIndent(em.episodes, "", "  ")
	os.WriteFile(path, data, 0644)
}

func (em *EpisodicMemory) load() {
	if em.dir == "" {
		return
	}
	path := filepath.Join(em.dir, "episodes.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &em.episodes)
}

func containsTag(tags []string, query string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}

// --- Memory System (unified) ---

// MemorySystem bundles all memory layers.
type MemorySystem struct {
	Working   *WorkingMemory
	Episodic  *EpisodicMemory
	Semantic  *RAGIndex           // TF-IDF code search (existing)
	SemMem    *SemanticMemory     // knowledge facts
	Extractor KnowledgeExtractor  // async fact extraction
}

// NewMemorySystem creates the full memory system.
// store may be nil for in-memory-only mode.
func NewMemorySystem(ragDir, episodicDir string, store StorageBackend) *MemorySystem {
	return &MemorySystem{
		Working:   NewWorkingMemory(100),
		Episodic:  NewEpisodicMemory(episodicDir, 1000, store),
		Semantic:  NewRAGIndex(),
		SemMem:    NewSemanticMemory(store),
		Extractor: &SimpleExtractor{},
	}
}

// AddObservation adds to working memory.
func (ms *MemorySystem) AddObservation(source, content string, importance float64) {
	ms.Working.Add(Observation{Source: source, Content: content, Importance: importance})
}

// RecordEpisode records a completed action and triggers async knowledge extraction.
func (ms *MemorySystem) RecordEpisode(trigger, action, outcome, lesson string, tags []string) {
	ep := Episode{Trigger: trigger, Action: action, Outcome: outcome, Lesson: lesson, Tags: tags}
	ms.Episodic.Record(ep)

	// Async knowledge extraction
	if ms.Extractor != nil && ms.SemMem != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			facts, err := ms.Extractor.Extract(ctx, ep)
			if err == nil {
				for _, f := range facts {
					ms.SemMem.Add(f)
				}
				fmt.Fprintf(os.Stderr, "[semantic] extracted %d facts from episode %s\n", len(facts), ep.ID)
			}
		}()
	}
}

// RecallEpisodes searches episodic memory.
func (ms *MemorySystem) RecallEpisodes(query string, limit int) []Episode {
	return ms.Episodic.Recall(query, limit)
}

// SemanticRecall uses TF-IDF for semantic search over indexed documents.
func (ms *MemorySystem) SemanticRecall(query string, topK int) []SearchResult {
	return ms.Semantic.Search(query, topK)
}

// SemanticQuery searches extracted knowledge facts.
func (ms *MemorySystem) SemanticQuery(query string, limit int) []SemanticFact {
	if ms.SemMem == nil {
		return nil
	}
	return ms.SemMem.Query(query, limit)
}

// Ensure imports used
var _ = sort.Strings
