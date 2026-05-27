package core

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// RAGContextStrategy wraps a strategy with RAG document context injection.
// When the last user message is long enough, it searches the RAG pipeline
// and injects relevant documents into the AgentState before delegating.
type RAGContextStrategy struct {
	inner    AgentStrategy
	pipeline *VectorRAGPipeline
}

// NewRAGContextStrategy creates a RAG-enhanced strategy wrapper.
func NewRAGContextStrategy(inner AgentStrategy, pipeline *VectorRAGPipeline) *RAGContextStrategy {
	return &RAGContextStrategy{inner: inner, pipeline: pipeline}
}

// Name returns "rag-" + inner strategy name.
func (s *RAGContextStrategy) Name() string { return "rag-" + s.inner.Name() }

// Decide checks for a user query, runs RAG search, and injects context.
func (s *RAGContextStrategy) Decide(ctx context.Context, state AgentState) (Decision, error) {
	if s.pipeline != nil && len(state.Messages) > 0 {
		lastMsg := state.Messages[len(state.Messages)-1]
		if lastMsg.Role == RoleUser && len(lastMsg.Content) > 10 {
			results, err := s.pipeline.Search(ctx, lastMsg.Content)
			if err == nil && len(results) > 0 {
				contextStr := s.pipeline.GenerateContext(results)
				// Log retrieval for observability
				scores := make([]string, len(results))
				queryEmb, _ := s.pipeline.embedder.Embed(ctx, lastMsg.Content)
				for i, r := range results {
					scores[i] = fmt.Sprintf("%.2f", CosineSimilarity(queryEmb, r.Embedding))
				}
				fmt.Fprintf(os.Stderr, "[rag] retrieved %d docs (scores: %s)\n", len(results), strings.Join(scores, "/"))

				// Prepend RAG context as a system message
				sysMsg := Message{Role: RoleSystem, Content: contextStr}
				state.Messages = append([]Message{sysMsg}, state.Messages...)

				// Add context to observations
				state.Observations = append(state.Observations,
					fmt.Sprintf("RAG: retrieved %d relevant documents", len(results)))
			}
		}
	}
	return s.inner.Decide(ctx, state)
}

func init() {
	RegisterStrategy("rag", func() AgentStrategy {
		embedder := GetEmbedder("tf-idf")
		// nil store means the strategy is registered but not active until store is wired
		var store VectorStore = nil
		pipeline := NewVectorRAGPipeline(embedder, store, 5)
		return NewRAGContextStrategy(NewReActStrategy(), pipeline)
	})
}
