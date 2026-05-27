package core

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Embedder Tests ---

func TestTFIDFEmbedder(t *testing.T) {
	e := NewTFIDFEmbedder()
	emb, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(emb) != e.Dimensions() {
		t.Errorf("expected %d dims, got %d", e.Dimensions(), len(emb))
	}
	// Check L2 normalization
	var norm float64
	for _, v := range emb {
		norm += v * v
	}
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("expected unit norm, got %f", norm)
	}
}

func TestTFIDFEmbedderEmpty(t *testing.T) {
	e := NewTFIDFEmbedder()
	emb, err := e.Embed(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(emb) != e.Dimensions() {
		t.Errorf("empty should still return vector of dim %d, got %d", e.Dimensions(), len(emb))
	}
	for _, v := range emb {
		if v != 0 {
			t.Error("empty text should produce zero vector")
			break
		}
	}
}

func TestTFIDFEmbedderConsistent(t *testing.T) {
	e := NewTFIDFEmbedder()
	a, _ := e.Embed(context.Background(), "hello world")
	b, _ := e.Embed(context.Background(), "hello world")
	if CosineSimilarity(a, b) < 0.99 {
		t.Error("same text should produce near-identical embeddings")
	}
}

func TestTFIDFEmbedderCustomDim(t *testing.T) {
	e := NewTFIDFEmbedderWithDim(512)
	if e.Dimensions() != 512 {
		t.Errorf("expected 512 dims, got %d", e.Dimensions())
	}
}

// --- Cosine Similarity Tests ---

func TestCosineSimilarity(t *testing.T) {
	a := Embedding{1, 0, 0}
	b := Embedding{0, 1, 0}
	if CosineSimilarity(a, b) != 0 {
		t.Error("orthogonal should be 0")
	}
	if CosineSimilarity(a, a) != 1 {
		t.Error("same should be 1")
	}
}

func TestCosineSimilarityNegative(t *testing.T) {
	a := Embedding{1, 0}
	b := Embedding{-1, 0}
	s := CosineSimilarity(a, b)
	if s > -0.99 || s < -1.01 {
		t.Errorf("opposite vectors should be -1, got %f", s)
	}
}

func TestCosineSimilarityZero(t *testing.T) {
	a := Embedding{0, 0, 0}
	if CosineSimilarity(a, a) != 0 {
		t.Error("zero vectors should return 0")
	}
}

func TestCosineSimilarityDifferentLengths(t *testing.T) {
	a := Embedding{1, 2, 3}
	b := Embedding{1, 2}
	if CosineSimilarity(a, b) != 0 {
		t.Error("different length vectors should return 0")
	}
}

// --- Document Loader Tests ---

func TestDocumentLoader(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hello\nWorld\n"), 0644)

	loader := NewDocumentLoader()
	chunks, err := loader.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Error("expected chunks from .go and .md files")
	}
	for _, c := range chunks {
		if c.Language != "go" && c.Language != "md" {
			t.Errorf("unexpected language: %s", c.Language)
		}
	}
}

func TestDocumentLoaderSkipBinary(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "image.png"), []byte{0x89, 0x50, 0x4E, 0x47}, 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hi\n"), 0644)

	loader := NewDocumentLoader()
	chunks, _ := loader.Load(dir)
	for _, c := range chunks {
		if strings.HasSuffix(c.Path, ".png") {
			t.Error("should skip binary files")
		}
	}
}

func TestDocumentLoaderSkipDirectories(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]\n"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hi\n"), 0644)

	loader := NewDocumentLoader()
	chunks, _ := loader.Load(dir)
	for _, c := range chunks {
		if strings.Contains(c.Path, ".git") || strings.Contains(c.Path, "node_modules") {
			t.Errorf("should skip .git and node_modules, got: %s", c.Path)
		}
	}
}

func TestDocumentLoaderEmptyDir(t *testing.T) {
	dir := t.TempDir()
	loader := NewDocumentLoader()
	chunks, err := loader.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Error("expected 0 chunks from empty dir")
	}
}

func TestDocumentLoaderMaxChunks(t *testing.T) {
	dir := t.TempDir()
	// Create a single file with content < ChunkSize (only 1 chunk).
	// MaxChunks=0 should still process it.
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0644)

	loader := NewDocumentLoader()
	loader.MaxChunks = 0
	chunks, _ := loader.Load(dir)
	// With MaxChunks=0, the first file's chunks are added, then SkipAll fires.
	// Since we already built chunks, they are kept.
	if len(chunks) < 1 {
		t.Error("expected at least 1 chunk from the file")
	}
}

// --- VectorRAGPipeline Tests ---

func TestVectorRAGPipelineIndex(t *testing.T) {
	store := createMemoryStore(t)
	embedder := NewTFIDFEmbedder()
	pipeline := NewVectorRAGPipeline(embedder, store, 5)

	chunks := []DocumentChunk{
		{ID: "doc1", Path: "auth.go", Content: "func authenticate(user string) bool { return true }", Language: "go"},
		{ID: "doc2", Path: "db.go", Content: "func connectDB() { sql.Open(...) }", Language: "go"},
		{ID: "doc3", Path: "test.go", Content: "func TestAuth(t *testing.T) { }", Language: "go"},
	}
	n, err := pipeline.Index(context.Background(), chunks)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Errorf("expected >0 indexed, got %d", n)
	}
	if store.Count() != 3 {
		t.Errorf("expected 3 docs, got %d", store.Count())
	}
}

func TestVectorRAGPipelineSearch(t *testing.T) {
	store := createMemoryStore(t)
	embedder := NewTFIDFEmbedder()
	pipeline := NewVectorRAGPipeline(embedder, store, 3)

	chunks := []DocumentChunk{
		{ID: "auth", Path: "auth.go", Content: "func authenticate user login password session token", Language: "go"},
		{ID: "db", Path: "db.go", Content: "database connection pool sqlite open driver", Language: "go"},
	}
	pipeline.Index(context.Background(), chunks)
	results, err := pipeline.Search(context.Background(), "user authentication")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if !strings.Contains(results[0].Chunk, "auth") {
		t.Errorf("expected auth first, got %s", results[0].Chunk)
	}
}

func TestVectorRAGPipelineEmptyStore(t *testing.T) {
	store := createMemoryStore(t)
	embedder := NewTFIDFEmbedder()
	pipeline := NewVectorRAGPipeline(embedder, store, 5)
	results, err := pipeline.Search(context.Background(), "anything")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Error("expected 0 results from empty store")
	}
}

func TestRAGContextGeneration(t *testing.T) {
	pipeline := NewVectorRAGPipeline(NewTFIDFEmbedder(), nil, 5)
	results := []VectorEntry{
		{Chunk: "auth.go", Text: "func authenticate..."},
		{Chunk: "login.go", Text: "func login..."},
	}
	ctx := pipeline.GenerateContext(results)
	if !strings.Contains(ctx, "auth.go") {
		t.Error("expected auth.go in context")
	}
	if !strings.Contains(ctx, "## Retrieved Documents") {
		t.Error("expected header")
	}
}

func TestRAGContextGenerationEmpty(t *testing.T) {
	pipeline := NewVectorRAGPipeline(NewTFIDFEmbedder(), nil, 5)
	ctx := pipeline.GenerateContext(nil)
	if ctx != "" {
		t.Error("expected empty context for nil results")
	}
}

func TestVectorRAGPipelineStats(t *testing.T) {
	store := createMemoryStore(t)
	pipeline := NewVectorRAGPipeline(NewTFIDFEmbedder(), store, 3)
	chunks := []DocumentChunk{
		{ID: "a", Path: "a.go", Content: "hello", Language: "go"},
	}
	pipeline.Index(context.Background(), chunks)
	stats := pipeline.Stats()
	if stats["documents"] != 1 {
		t.Errorf("expected 1 document, got %d", stats["documents"])
	}
	if stats["top_k"] != 3 {
		t.Errorf("expected top_k=3, got %d", stats["top_k"])
	}
}

// --- Backward Compatibility ---

func TestRAGBackwardCompat(t *testing.T) {
	// Existing TF-IDF RAGIndex should still work independently
	idx := NewRAGIndex()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.go"), []byte("func hello() {}"), 0644)
	idx.IndexWorkspace(dir, []string{".go"})
	results := idx.Search("hello", 3)
	if len(results) == 0 {
		t.Error("existing RAGIndex should still work")
	}
}

// --- Embedder Registry ---

func TestEmbedderRegistry(t *testing.T) {
	e := GetEmbedder("tf-idf")
	if e == nil {
		t.Fatal("tf-idf embedder should be registered")
	}
	if e.Name() != "tf-idf" {
		t.Errorf("expected tf-idf, got %s", e.Name())
	}
	e2 := GetEmbedder("nonexistent")
	if e2 == nil {
		t.Fatal("unknown embedder name should fall back to default")
	}
	if e2.Name() != "tf-idf" {
		t.Errorf("expected fallback to tf-idf, got %s", e2.Name())
	}
}

func TestEmbedderRegistryCustom(t *testing.T) {
	e := NewTFIDFEmbedderWithDim(128)
	RegisterEmbedder(e)
	if GetEmbedder("tf-idf").Dimensions() != 128 {
		t.Error("re-registration should update")
	}
}

// --- RAG Strategy Registration ---

func TestRAGStrategyRegistration(t *testing.T) {
	s := GetStrategy("rag")
	if s == nil {
		t.Fatal("rag strategy should be registered")
	}
	if !strings.Contains(s.Name(), "rag") {
		t.Errorf("expected 'rag' in name, got %s", s.Name())
	}
}

// --- RAG Zero Overhead ---

func TestRAGZeroOverhead(t *testing.T) {
	store := createMemoryStore(t)
	if store.Count() != 0 {
		t.Error("empty store should have 0 docs")
	}
}

// --- Helper ---

// createMemoryStore creates an isolated SQLiteVectorStore for testing.
func createMemoryStore(t *testing.T) *SQLiteVectorStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vec.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	vs, err := NewSQLiteVectorStore(store)
	if err != nil {
		t.Fatal(err)
	}
	return vs
}
