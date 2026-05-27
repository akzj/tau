package core

import (
	"fmt"
	"sync"
)

// ContextBudget tracks token usage against a maximum.
type ContextBudget struct {
	mu              sync.Mutex
	MaxTokens       int
	UsedTokens      int
	ReservedTokens  int // tokens reserved for system prompt + tools
	CompressionCount int
}

// NewContextBudget creates a token budget tracker.
func NewContextBudget(maxTokens int) *ContextBudget {
	if maxTokens <= 0 {
		maxTokens = 128000
	}
	return &ContextBudget{MaxTokens: maxTokens, ReservedTokens: 2000}
}


// EstimateTokensPrecise uses a more precise heuristic (tiktoken fallback).
func EstimateTokensPrecise(text string) int {
	// Simple but more accurate: 1 token ≈ 3.5 chars for English, 1.5 for CJK
	chars := 0
	cjk := 0
	for _, r := range text {
		chars++
		if r >= 0x4E00 && r <= 0x9FFF {
			cjk++
		}
	}
	if chars == 0 {
		return 0
	}
	if float64(cjk)/float64(chars) > 0.3 {
		return chars * 2 / 3
	}
	return chars * 2 / 7
}

// CanFit checks if a message can fit within the budget.
func (cb *ContextBudget) CanFit(msg string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.UsedTokens+EstimateTokensPrecise(msg)+cb.ReservedTokens <= cb.MaxTokens
}

// Reserve subtracts tokens from the budget for a message.
func (cb *ContextBudget) Reserve(tokens int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.UsedTokens += tokens
}

// Remaining returns available token capacity.
func (cb *ContextBudget) Remaining() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	remaining := cb.MaxTokens - cb.UsedTokens - cb.ReservedTokens
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

// Reset clears usage counter.
func (cb *ContextBudget) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.UsedTokens = 0
	cb.CompressionCount = 0
}

// Usage returns used/max/reserved as a formatted string.
func (cb *ContextBudget) Usage() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	pct := float64(cb.UsedTokens) / float64(cb.MaxTokens) * 100
	return fmt.Sprintf("%d/%d tokens (%.1f%%, compression count: %d)",
		cb.UsedTokens, cb.MaxTokens, pct, cb.CompressionCount)
}

// Stats returns detailed stats for visualization.
func (cb *ContextBudget) Stats() map[string]int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return map[string]int{
		"max":          cb.MaxTokens,
		"used":         cb.UsedTokens,
		"reserved":     cb.ReservedTokens,
		"remaining":    cb.MaxTokens - cb.UsedTokens - cb.ReservedTokens,
		"compressions": cb.CompressionCount,
	}
}
