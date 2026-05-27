package core

import (
	"fmt"
	"os"
	"strings"
)

// CompressionConfig controls how conversation compression behaves.
type CompressionConfig struct {
	Threshold   float64 // 0.0-1.0, when to trigger compression (default 0.8)
	TargetRatio float64 // compress to what ratio (default 0.5)
	KeepSystem  bool    // preserve system messages (default true)
	KeepErrors  bool    // preserve error messages (default true)
	KeepRecent  int     // preserve N most recent messages (default 3)
	MaxMessages int     // max messages to keep after compression (default 20)
	Strategy    string  // "sliding", "summarize", "truncate"
}

// DefaultCompressionConfig returns sensible defaults.
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Threshold:   0.8,
		TargetRatio: 0.5,
		KeepSystem:  true,
		KeepErrors:  true,
		KeepRecent:  3,
		MaxMessages: 20,
		Strategy:    "sliding",
	}
}

// ConversationCompressor compresses conversation history to fit token budgets.
type ConversationCompressor struct {
	config CompressionConfig
	budget *ContextBudget
}

// NewConversationCompressor creates a compressor.
func NewConversationCompressor(config CompressionConfig, budget *ContextBudget) *ConversationCompressor {
	return &ConversationCompressor{config: config, budget: budget}
}

// Compress reduces a message list to fit within budget constraints.
// Returns the compressed message list and a summary of what was done.
func (cc *ConversationCompressor) Compress(messages []Message) ([]Message, string) {
	if cc.budget == nil {
		return messages, ""
	}

	totalTokens := cc.countTokens(messages)
	if float64(totalTokens) <= float64(cc.budget.MaxTokens)*cc.config.Threshold {
		return messages, "" // no compression needed
	}

	before := len(messages)
	beforeTokens := totalTokens

	switch cc.config.Strategy {
	case "summarize":
		messages = cc.compressBySummarize(messages)
	case "truncate":
		messages = cc.compressByTruncate(messages)
	default: // sliding window
		messages = cc.compressSliding(messages)
	}

	after := len(messages)
	afterTokens := cc.countTokens(messages)
	saved := beforeTokens - afterTokens

	cc.budget.mu.Lock()
	cc.budget.CompressionCount++
	cc.budget.UsedTokens = afterTokens
	cc.budget.mu.Unlock()

	summary := fmt.Sprintf("[context] compressed %d→%d messages (%d tokens saved, strategy: %s)",
		before, after, saved, cc.config.Strategy)
	fmt.Fprintf(os.Stderr, "%s\n", summary)
	return messages, summary
}

func (cc *ConversationCompressor) compressSliding(messages []Message) []Message {
	// Priority: system > errors > most recent N
	var system, errors, recent, old []Message
	for i, m := range messages {
		switch {
		case cc.config.KeepSystem && m.Role == RoleSystem:
			system = append(system, m)
		case cc.config.KeepErrors && isErrorMessage(m):
			errors = append(errors, m)
		case i >= len(messages)-cc.config.KeepRecent:
			recent = append(recent, m)
		default:
			old = append(old, m)
		}
	}
	_ = old // old messages are discarded

	// Build result: system + errors + recent
	result := append(system, errors...)
	keepCount := cc.config.MaxMessages - len(system) - len(errors)
	if keepCount < 5 {
		keepCount = 5
	}

	// Add recent messages
	if len(recent) > keepCount {
		recent = recent[len(recent)-keepCount:]
	}
	result = append(result, recent...)

	return result
}

func (cc *ConversationCompressor) compressByTruncate(messages []Message) []Message {
	if len(messages) <= cc.config.MaxMessages {
		return messages
	}
	return messages[len(messages)-cc.config.MaxMessages:]
}

func (cc *ConversationCompressor) compressBySummarize(messages []Message) []Message {
	// Keep system + last N, replace middle with a summary marker
	var result []Message
	for _, m := range messages {
		if cc.config.KeepSystem && m.Role == RoleSystem {
			result = append(result, m)
		}
	}
	result = append(result, Message{
		Role:    RoleSystem,
		Content: fmt.Sprintf("[Context: %d earlier messages summarized away]", len(messages)-cc.config.KeepRecent-1),
	})
	if len(messages) > cc.config.KeepRecent {
		result = append(result, messages[len(messages)-cc.config.KeepRecent:]...)
	}
	return result
}

func (cc *ConversationCompressor) countTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += EstimateTokensPrecise(m.Content)
	}
	return total
}

func isErrorMessage(m Message) bool {
	lower := strings.ToLower(m.Content)
	return strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "timeout")
}
