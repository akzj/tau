package core

import (
	"context"
	"strings"
)

// ContextControlStrategy wraps a strategy with context window management.
type ContextControlStrategy struct {
	inner      AgentStrategy
	compressor *ConversationCompressor
	budget     *ContextBudget
}

// NewContextControlStrategy creates a context-aware strategy wrapper.
func NewContextControlStrategy(inner AgentStrategy, config CompressionConfig, budget *ContextBudget) *ContextControlStrategy {
	return &ContextControlStrategy{
		inner:      inner,
		compressor: NewConversationCompressor(config, budget),
		budget:     budget,
	}
}

// Name returns "context-" + inner strategy name.
func (s *ContextControlStrategy) Name() string { return "context-" + s.inner.Name() }

// Decide checks context budget before delegating to inner strategy.
func (s *ContextControlStrategy) Decide(ctx context.Context, state AgentState) (Decision, error) {
	// Check context budget before deciding
	if len(state.Messages) > 0 {
		totalTokens := EstimateTokensPrecise(joinMessages(state.Messages))
		if float64(totalTokens) > float64(s.budget.MaxTokens)*DefaultCompressionConfig().Threshold {
			// Compress and update state
			compressed, summary := s.compressor.Compress(state.Messages)
			state.Messages = compressed
			return Decision{
				Action: "continue",
				Reason: summary,
			}, nil
		}
	}
	return s.inner.Decide(ctx, state)
}

func joinMessages(msgs []Message) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.Content)
		b.WriteString(" ")
	}
	return b.String()
}

func init() {
	RegisterStrategy("context", func() AgentStrategy {
		config := DefaultCompressionConfig()
		budget := NewContextBudget(128000)
		return NewContextControlStrategy(NewReActStrategy(), config, budget)
	})
}
