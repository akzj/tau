package cache_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/core/cache"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newBackend(t *testing.T, maxSize int) cache.CacheBackend {
	t.Helper()
	c, err := cache.NewSQLiteCache(openDB(t), maxSize)
	if err != nil {
		t.Fatalf("NewSQLiteCache: %v", err)
	}
	return c
}

// ---------------------------------------------------------------------------
// fake provider / embedder / tool
// ---------------------------------------------------------------------------

type fakeProvider struct {
	streamResp  []core.ProviderEvent
	streamErr   error
	completeResp core.CompleteResponse
	completeErr error

	streamCalls    int
	completeCalls  int
}

func (f *fakeProvider) Stream(_ context.Context, _ core.StreamRequest) (<-chan core.ProviderEvent, error) {
	f.streamCalls++
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	ch := make(chan core.ProviderEvent, len(f.streamResp))
	for _, ev := range f.streamResp {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func (f *fakeProvider) Complete(_ context.Context, _ core.CompleteRequest) (core.CompleteResponse, error) {
	f.completeCalls++
	return f.completeResp, f.completeErr
}

type fakeEmbedder struct {
	name       string
	dimensions int
	response   core.Embedding
	err        error
	calls      int
}

func (f *fakeEmbedder) Embed(_ context.Context, _ string) (core.Embedding, error) {
	f.calls++
	return f.response, f.err
}
func (f *fakeEmbedder) Name() string    { return f.name }
func (f *fakeEmbedder) Dimensions() int { return f.dimensions }

// ---------------------------------------------------------------------------
// CacheBackend tests
// ---------------------------------------------------------------------------

func TestCacheCRUD(t *testing.T) {
	b := newBackend(t, 0)

	_, ok := b.Get("missing")
	if ok {
		t.Fatal("expected miss for missing key")
	}

	b.Set("k1", []byte("hello"), time.Minute)
	v, ok := b.Get("k1")
	if !ok || string(v) != "hello" {
		t.Fatalf("got %q, %v; want hello, true", v, ok)
	}

	b.Delete("k1")
	_, ok = b.Get("k1")
	if ok {
		t.Fatal("expected miss after delete")
	}
}

func TestCacheTTL(t *testing.T) {
	b := newBackend(t, 0)

	b.Set("short", []byte("expires"), 50*time.Millisecond)

	v, ok := b.Get("short")
	if !ok || string(v) != "expires" {
		t.Fatalf("immediate get failed: %q, %v", v, ok)
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = b.Get("short")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestCacheStats(t *testing.T) {
	b := newBackend(t, 0)

	// miss → hit
	b.Set("a", []byte("1"), time.Minute)
	b.Get("missing") // miss
	b.Get("a")       // hit
	b.Get("a")       // hit

	s := b.Stats()
	if s.Hits != 2 {
		t.Errorf("hits: got %d, want 2", s.Hits)
	}
	if s.Misses != 1 {
		t.Errorf("misses: got %d, want 1", s.Misses)
	}
	if s.ItemCount != 1 {
		t.Errorf("items: got %d, want 1", s.ItemCount)
	}
}

func TestCacheMaxSize(t *testing.T) {
	b := newBackend(t, 3)

	for i := range 5 {
		b.Set(string(rune('a'+i)), []byte("x"), 10*time.Minute)
	}

	s := b.Stats()
	if s.ItemCount > 3 {
		t.Errorf("maxSize not enforced: %d items (max 3)", s.ItemCount)
	}
}

func TestCacheFlush(t *testing.T) {
	b := newBackend(t, 0)

	b.Set("a", []byte("1"), time.Minute)
	b.Set("b", []byte("2"), time.Minute)

	// Stats right after Flush — everything zero
	b.Flush()
	s := b.Stats()
	if s.ItemCount != 0 || s.Hits != 0 || s.Misses != 0 {
		t.Errorf("stats not reset after flush: %+v", s)
	}

	// Subsequent Get on flushed cache is a miss
	_, ok := b.Get("a")
	if ok {
		t.Fatal("expected miss after flush")
	}
}

func TestCacheNilDB(t *testing.T) {
	_, err := cache.NewSQLiteCache(nil, 0)
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

// ---------------------------------------------------------------------------
// LLM cache tests
// ---------------------------------------------------------------------------

func TestLLMCacheHit(t *testing.T) {
	b := newBackend(t, 0)
	fp := &fakeProvider{
		streamResp: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "world"},
			{Type: core.ProvMessageEnd},
		},
	}
	mw := cache.NewLLMCache(fp, b, 0)

	req := core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	}

	// First call — miss, populates cache
	ch1, err := mw.Stream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out1 := drainStream(t, ch1)
	if out1 != "world" {
		t.Fatalf("first call: got %q, want world", out1)
	}
	if fp.streamCalls != 1 {
		t.Errorf("streamCalls: got %d, want 1", fp.streamCalls)
	}

	// Second call — should be cache hit
	ch2, err := mw.Stream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out2 := drainStream(t, ch2)
	if out2 != "world" {
		t.Fatalf("cached call: got %q, want world", out2)
	}
	// No additional call to inner provider
	if fp.streamCalls != 1 {
		t.Errorf("streamCalls after hit: got %d, want 1", fp.streamCalls)
	}
}

func TestLLMCacheMiss(t *testing.T) {
	b := newBackend(t, 0)
	fp := &fakeProvider{
		streamResp: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "alpha"},
			{Type: core.ProvMessageEnd},
		},
	}
	mw := cache.NewLLMCache(fp, b, 0)

	req := core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hello"}},
	}

	ch, err := mw.Stream(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out := drainStream(t, ch)
	if out != "alpha" {
		t.Fatalf("got %q, want alpha", out)
	}

	// Different request → different key → cache miss → inner called again
	req2 := core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "bye"}},
	}
	ch2, err := mw.Stream(context.Background(), req2)
	if err != nil {
		t.Fatal(err)
	}
	drainStream(t, ch2)
	if fp.streamCalls != 2 {
		t.Errorf("streamCalls: got %d, want 2", fp.streamCalls)
	}
}

func TestLLMCacheDisabled(t *testing.T) {
	b := newBackend(t, 0)
	fp := &fakeProvider{
		streamResp: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "data"},
			{Type: core.ProvMessageEnd},
		},
	}
	mw := cache.NewLLMCache(fp, b, 0)
	mw.SetEnabled(false)

	req := core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "q"}},
	}

	ch1, _ := mw.Stream(context.Background(), req)
	drainStream(t, ch1)
	ch2, _ := mw.Stream(context.Background(), req)
	drainStream(t, ch2)

	if fp.streamCalls != 2 {
		t.Errorf("disabled: streamCalls got %d, want 2", fp.streamCalls)
	}
}

func TestLLMCacheComplete(t *testing.T) {
	b := newBackend(t, 0)
	fp := &fakeProvider{
		completeResp: core.CompleteResponse{Content: "done"},
	}
	mw := cache.NewLLMCache(fp, b, 0)

	req := core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "x"}},
	}

	resp1, err := mw.Complete(context.Background(), req)
	if err != nil || resp1.Content != "done" {
		t.Fatalf("first Complete: %v, %q", err, resp1.Content)
	}
	resp2, err := mw.Complete(context.Background(), req)
	if err != nil || resp2.Content != "done" {
		t.Fatalf("cached Complete: %v, %q", err, resp2.Content)
	}
	if fp.completeCalls != 1 {
		t.Errorf("completeCalls: got %d, want 1", fp.completeCalls)
	}
}

// ---------------------------------------------------------------------------
// Embedding cache tests
// ---------------------------------------------------------------------------

func TestEmbeddingCacheHit(t *testing.T) {
	b := newBackend(t, 0)
	fe := &fakeEmbedder{
		name:       "test-emb",
		dimensions: 3,
		response:   core.Embedding{0.1, 0.2, 0.3},
	}
	mw := cache.NewEmbeddingCache(fe, b)

	ctx := context.Background()

	emb1, err := mw.Embed(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(emb1) != 3 {
		t.Fatalf("got %d dims", len(emb1))
	}
	if fe.calls != 1 {
		t.Errorf("first call: calls got %d, want 1", fe.calls)
	}

	// Same text — cache hit
	emb2, err := mw.Embed(ctx, "hello")
	if err != nil || len(emb2) != 3 {
		t.Fatalf("cached: %v, %d dims", err, len(emb2))
	}
	if fe.calls != 1 {
		t.Errorf("after hit: calls got %d, want 1", fe.calls)
	}

	// Different text — miss
	emb3, err := mw.Embed(ctx, "world")
	if err != nil || len(emb3) != 3 {
		t.Fatalf("miss: %v", err)
	}
	if fe.calls != 2 {
		t.Errorf("after miss: calls got %d, want 2", fe.calls)
	}
}

func TestEmbeddingCacheDisabled(t *testing.T) {
	b := newBackend(t, 0)
	fe := &fakeEmbedder{name: "e", dimensions: 2, response: core.Embedding{1, 2}}
	mw := cache.NewEmbeddingCache(fe, b)
	mw.SetEnabled(false)

	mw.Embed(context.Background(), "a")
	mw.Embed(context.Background(), "a")
	if fe.calls != 2 {
		t.Errorf("disabled: calls got %d, want 2", fe.calls)
	}
}

func TestEmbeddingCachePassthrough(t *testing.T) {
	b := newBackend(t, 0)
	fe := &fakeEmbedder{name: "e2", dimensions: 16, response: core.Embedding{0.5}}
	mw := cache.NewEmbeddingCache(fe, b)

	if mw.Name() != "e2" {
		t.Errorf("Name: got %q, want e2", mw.Name())
	}
	if mw.Dimensions() != 16 {
		t.Errorf("Dimensions: got %d, want 16", mw.Dimensions())
	}
}

// ---------------------------------------------------------------------------
// Tool cache tests
// ---------------------------------------------------------------------------

func TestToolCacheHit(t *testing.T) {
	b := newBackend(t, 0)
	mw := cache.NewToolCache(b)

	var execCalls int
	tool := core.Tool{
		Name: "grep",
		Execute: func(_ context.Context, _ string, _ any, _ func(core.PartialResult)) (core.ToolResult, error) {
			execCalls++
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: "found: 3 matches"}},
			}, nil
		},
	}

	wrapped := mw.WrapTool(tool)
	ctx := context.Background()

	_, err := wrapped.Execute(ctx, "c1", map[string]any{"pattern": "foo"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if execCalls != 1 {
		t.Errorf("first call: execCalls got %d, want 1", execCalls)
	}

	// Same params → cache hit
	r2, err := wrapped.Execute(ctx, "c2", map[string]any{"pattern": "foo"}, nil)
	if err != nil || r2.Content[0].Text != "found: 3 matches" {
		t.Fatalf("cached: %v", err)
	}
	if execCalls != 1 {
		t.Errorf("after hit: execCalls got %d, want 1", execCalls)
	}

	// Different params → miss
	_, _ = wrapped.Execute(ctx, "c3", map[string]any{"pattern": "bar"}, nil)
	if execCalls != 2 {
		t.Errorf("after miss: execCalls got %d, want 2", execCalls)
	}
}

func TestToolCacheDisabled(t *testing.T) {
	b := newBackend(t, 0)
	mw := cache.NewToolCache(b)
	mw.SetEnabled(false)

	var calls int
	tool := core.Tool{
		Name: "test",
		Execute: func(_ context.Context, _ string, _ any, _ func(core.PartialResult)) (core.ToolResult, error) {
			calls++
			return core.ToolResult{}, nil
		},
	}
	wrapped := mw.WrapTool(tool)

	wrapped.Execute(context.Background(), "x", nil, nil)
	wrapped.Execute(context.Background(), "x", nil, nil)
	if calls != 2 {
		t.Errorf("disabled: calls got %d, want 2", calls)
	}
}

func TestToolCachePerToolTTL(t *testing.T) {
	b := newBackend(t, 0)
	mw := cache.NewToolCache(b)

	var calls int
	tool := core.Tool{
		Name: "git_log",
		Execute: func(_ context.Context, _ string, _ any, _ func(core.PartialResult)) (core.ToolResult, error) {
			calls++
			return core.ToolResult{Content: []core.Content{{Text: "log"}}}, nil
		},
	}
	wrapped := mw.WrapTool(tool)
	ctx := context.Background()

	wrapped.Execute(ctx, "c1", "HEAD", nil) // miss
	wrapped.Execute(ctx, "c2", "HEAD", nil) // hit (30s TTL)
	if calls != 1 {
		t.Errorf("git_log cache: calls got %d, want 1", calls)
	}
}

func TestToolCacheNonJSONParams(t *testing.T) {
	b := newBackend(t, 0)
	mw := cache.NewToolCache(b)

	var calls int
	tool := core.Tool{
		Name: "echo",
		Execute: func(_ context.Context, _ string, _ any, _ func(core.PartialResult)) (core.ToolResult, error) {
			calls++
			return core.ToolResult{}, nil
		},
	}
	wrapped := mw.WrapTool(tool)
	ctx := context.Background()

	// Chan params can't be JSON-marshaled → falls through to Execute
	ch := make(chan int)
	wrapped.Execute(ctx, "c1", ch, nil)
	wrapped.Execute(ctx, "c2", ch, nil)
	// Both should call Execute because marshal fails
	if calls != 2 {
		t.Errorf("non-JSON: calls got %d, want 2", calls)
	}
}

// ---------------------------------------------------------------------------
// CacheStats JSON round-trip
// ---------------------------------------------------------------------------

func TestCacheStatsJSON(t *testing.T) {
	b := newBackend(t, 0)
	b.Set("s", []byte("v"), time.Minute)
	b.Get("s")
	b.Get("miss")

	s := b.Stats()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal stats: %v", err)
	}
	var s2 cache.CacheStats
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
	if s2.Hits != s.Hits || s2.ItemCount != s.ItemCount {
		t.Errorf("round-trip mismatch: %+v → %+v", s, s2)
	}
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func TestConcurrentAccess(t *testing.T) {
	b := newBackend(t, 0)
	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := string(rune('a'+id)) + "-" + string(rune('0'+j%10))
				b.Set(key, []byte("v"), time.Minute)
			}
		}(i)
		go func(id int) {
			defer wg.Done()
			for range iterations {
				b.Get("a-0")
				b.Stats()
			}
		}(i)
	}

	wg.Wait()
	// No deadlock or panic = pass
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

func drainStream(t *testing.T, ch <-chan core.ProviderEvent) string {
	t.Helper()
	var result string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			result += ev.ContentDelta
		}
	}
	return result
}
