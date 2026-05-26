package core

import (
	"unicode/utf8"
)

// TokenUsage tracks token consumption and cost for a single call.
type TokenUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Model            string  `json:"model"`
	CostUSD          float64 `json:"cost_usd"`
}

// modelCosts maps model IDs to cost per 1M tokens (input, output).
var modelCosts = map[string][2]float64{
	"gpt-5.4":           {2.50, 10.00},
	"gpt-4o":            {2.50, 10.00},
	"gpt-4o-mini":       {0.15, 0.60},
	"gpt-4":             {30.00, 60.00},
	"o1":                {15.00, 60.00},
	"o1-mini":           {1.10, 4.40},
	"claude-sonnet-4-6": {3.00, 15.00},
	"claude-haiku-3-5":  {0.80, 4.00},
	"claude-opus-4":     {15.00, 75.00},
	"gemini-2.5-flash":  {0.15, 0.60},
	"gemini-2.5-pro":    {1.25, 5.00},
	"mistral-large":     {2.00, 6.00},
	"mistral-small":     {0.20, 0.60},
	"codestral":         {0.30, 0.90},
}

// CountTokens estimates the number of tokens in a string.
func CountTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	chars := 0
	cjk := 0
	for _, r := range text {
		chars++
		if isCJK(r) {
			cjk++
		}
	}

	if float64(cjk)/float64(chars) > 0.3 {
		return int(float64(chars) / 1.5)
	}
	return chars / 4
}

// isCJK returns true if the rune is in a CJK range.
func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0x2E80 && r <= 0x2EFF) || // CJK Radicals
		(r >= 0x3000 && r <= 0x303F) || // CJK Symbols
		(r >= 0xFF00 && r <= 0xFFEF) || // Half/Full-width
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul
}

// CountMessages estimates total tokens across messages.
func CountMessages(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += CountTokens(m.Content)
		for _, tc := range m.ToolCalls {
			total += CountTokens(tc.Args) + CountTokens(tc.ToolName)
		}
	}
	return total
}

// EstimateCost computes cost based on model and token counts.
func EstimateCost(model string, promptTokens, completionTokens int) float64 {
	costs, ok := modelCosts[model]
	if !ok {
		costs = [2]float64{2.50, 10.00} // default: GPT-4o pricing
	}
	inputCost := float64(promptTokens) / 1_000_000 * costs[0]
	outputCost := float64(completionTokens) / 1_000_000 * costs[1]
	return inputCost + outputCost
}

// NewTokenUsage creates a TokenUsage with cost estimation.
func NewTokenUsage(model string, promptTokens, completionTokens int) TokenUsage {
	return TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		Model:            model,
		CostUSD:          EstimateCost(model, promptTokens, completionTokens),
	}
}

// Accumulate adds usage to a cumulative total.
func (u *TokenUsage) Accumulate(other TokenUsage) {
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
	u.CostUSD += other.CostUSD
}

// Ensure utf8 import is used
var _ = utf8.RuneCountInString
