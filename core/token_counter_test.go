package core

import (
	"testing"
)

func TestCountTokensEnglish(t *testing.T) {
	text := "hello world this is a test message"
	tokens := CountTokens(text)
	expected := len(text) / 4
	if tokens != expected {
		t.Errorf("English: expected %d, got %d", expected, tokens)
	}
}

func TestCountTokensCJK(t *testing.T) {
	text := "你好世界这是一个测试消息"
	tokens := CountTokens(text)
	if tokens < 5 || tokens > 9 {
		t.Errorf("CJK: expected ~7, got %d", tokens)
	}
}

func TestCountTokensMixed(t *testing.T) {
	text := "hello 你好 world 世界"
	tokens := CountTokens(text)
	if tokens <= 0 {
		t.Error("mixed: expected >0 tokens")
	}
}

func TestCountMessages(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Content: "hello world"},
		{Role: RoleAssistant, Content: "hi there"},
	}
	tokens := CountMessages(msgs)
	if tokens <= 0 {
		t.Error("expected >0 tokens for messages")
	}
}

func TestEstimateCost(t *testing.T) {
	cost := EstimateCost("gpt-4o", 1000, 500)
	if cost <= 0 {
		t.Error("expected non-zero cost")
	}
	expected := 0.0025 + 0.005
	if cost < expected*0.9 || cost > expected*1.1 {
		t.Errorf("cost: expected ~%f, got %f", expected, cost)
	}
}

func TestAccumulate(t *testing.T) {
	u1 := NewTokenUsage("gpt-4o", 100, 50)
	u2 := NewTokenUsage("gpt-4o", 200, 100)
	u1.Accumulate(u2)
	if u1.PromptTokens != 300 {
		t.Errorf("prompt: expected 300, got %d", u1.PromptTokens)
	}
	if u1.CompletionTokens != 150 {
		t.Errorf("completion: expected 150, got %d", u1.CompletionTokens)
	}
	if u1.TotalTokens != 450 {
		t.Errorf("total: expected 450, got %d", u1.TotalTokens)
	}
}
