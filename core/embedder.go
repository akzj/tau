package core

import (
	"context"
	"math"
	"sync"
)

// Embedding is a dense vector representation of text.
type Embedding []float64

// Embedder generates embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) (Embedding, error)
	Name() string
	Dimensions() int
}

// EmbedderRegistry manages available embedders.
type EmbedderRegistry struct {
	mu        sync.RWMutex
	embedders map[string]Embedder
	default_  Embedder
}

var globalEmbedders = &EmbedderRegistry{embedders: make(map[string]Embedder)}

// RegisterEmbedder adds an embedder to the registry.
func RegisterEmbedder(e Embedder) {
	globalEmbedders.mu.Lock()
	defer globalEmbedders.mu.Unlock()
	globalEmbedders.embedders[e.Name()] = e
	if globalEmbedders.default_ == nil {
		globalEmbedders.default_ = e
	}
}

// GetEmbedder returns an embedder by name, or the default if not found.
func GetEmbedder(name string) Embedder {
	globalEmbedders.mu.RLock()
	defer globalEmbedders.mu.RUnlock()
	if e, ok := globalEmbedders.embedders[name]; ok {
		return e
	}
	return globalEmbedders.default_
}

// --- TF-IDF Embedder (zero API dependency) ---

// TFIDFEmbedder produces sparse vector embeddings using a bag-of-words approach
// with hash bucketing. This is the default fallback — zero external dependencies.
type TFIDFEmbedder struct {
	dim int
}

// NewTFIDFEmbedder creates a TF-IDF embedder with the default 256 dimensions.
func NewTFIDFEmbedder() *TFIDFEmbedder {
	return &TFIDFEmbedder{dim: 256}
}

// NewTFIDFEmbedderWithDim creates a TF-IDF embedder with a custom dimension count.
func NewTFIDFEmbedderWithDim(dim int) *TFIDFEmbedder {
	if dim <= 0 {
		dim = 256
	}
	return &TFIDFEmbedder{dim: dim}
}

// Name returns "tf-idf".
func (e *TFIDFEmbedder) Name() string { return "tf-idf" }

// Dimensions returns the embedding dimension.
func (e *TFIDFEmbedder) Dimensions() int { return e.dim }

// Embed tokenizes text and produces a hash-bucketed, L2-normalized vector.
func (e *TFIDFEmbedder) Embed(ctx context.Context, text string) (Embedding, error) {
	tokens := tokenize(text)
	vec := make(Embedding, e.dim)
	for _, t := range tokens {
		h := hash32(t) % uint32(e.dim)
		vec[h] += 1.0
	}
	// L2 normalize
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec, nil
}

// hash32 is a simple FNV-1a style hash for strings.
func hash32(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// --- Cosine Similarity ---

// CosineSimilarity computes the cosine similarity between two equal-length vectors.
// Returns 0 if vectors have different lengths or are zero-norm.
func CosineSimilarity(a, b Embedding) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func init() {
	RegisterEmbedder(NewTFIDFEmbedder())
}
