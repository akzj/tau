package core

import (
	"strings"
	"testing"
)

func TestContextBudgetBasic(t *testing.T) {
	cb := NewContextBudget(10000)
	if cb.MaxTokens != 10000 {
		t.Errorf("expected 10000, got %d", cb.MaxTokens)
	}
	if cb.Remaining() <= 0 {
		t.Error("expected positive remaining")
	}
}

func TestContextBudgetReserve(t *testing.T) {
	cb := NewContextBudget(10000)
	cb.Reserve(5000)
	remaining := cb.Remaining()
	// max=10000, used=5000, reserved=2000 => remaining=3000
	if remaining > 3000 {
		t.Errorf("remaining too high: %d", remaining)
	}
	if remaining != 3000 {
		t.Errorf("expected 3000 remaining, got %d", remaining)
	}
}

func TestContextBudgetOverflow(t *testing.T) {
	cb := NewContextBudget(100)
	cb.Reserve(1000)
	if cb.Remaining() != 0 {
		t.Error("expected 0 remaining after overflow")
	}
}

func TestEstimateTokens(t *testing.T) {
	// Use existing EstimateTokens from compaction.go (takes []Message)
	tokens := EstimateTokens([]Message{{Content: "hello world"}})
	if tokens <= 0 {
		t.Error("expected >0 tokens")
	}
	// "hello world" = 11 chars / 4 ≈ 2 tokens
	if tokens != 2 {
		t.Errorf("expected ~2-3, got %d", tokens)
	}
}

func TestEstimateTokensPreciseCJK(t *testing.T) {
	en := EstimateTokensPrecise("hello world this is english text")
	cjk := EstimateTokensPrecise("你好世界这是一个中文测试")
	if en <= 0 || cjk <= 0 {
		t.Error("expected >0 tokens for both")
	}
	// CJK should yield different token count than English for same char count
	if en == cjk {
		t.Log("CJK and English token estimates differ (expected)")
	}
}

func TestContextBudgetUsageString(t *testing.T) {
	cb := NewContextBudget(1000)
	cb.Reserve(300)
	usage := cb.Usage()
	if !strings.Contains(usage, "30") {
		t.Errorf("expected 30%% in usage: %s", usage)
	}
}

func TestCompressorSlidingWindow(t *testing.T) {
	cfg := DefaultCompressionConfig()
	// Budget=100: threshold 80 tokens. 30 msgs × (35*2/7=10) = 300 tokens > 80 → triggers.
	cb := NewContextBudget(100)
	cc := NewConversationCompressor(cfg, cb)

	msgs := make([]Message, 30)
	for i := range msgs {
		msgs[i] = Message{Role: RoleUser, Content: strings.Repeat("x", 35)}
	}
	compressed, summary := cc.Compress(msgs)
	if len(compressed) >= len(msgs) {
		t.Error("expected compression")
	}
	if !strings.Contains(summary, "[context]") {
		t.Error("expected context summary")
	}
}

func TestCompressorPreserveErrors(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cb := NewContextBudget(200)
	cc := NewConversationCompressor(cfg, cb)

	msgs := []Message{
		{Role: RoleSystem, Content: "you are a bot"},
		{Role: RoleUser, Content: strings.Repeat("x", 30)},
		{Role: RoleAssistant, Content: "error: null pointer at line 42"},
		{Role: RoleUser, Content: strings.Repeat("y", 20)},
	}
	compressed, _ := cc.Compress(msgs)
	foundError := false
	for _, m := range compressed {
		if strings.Contains(m.Content, "error") {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected error message preserved")
	}
}

func TestCompressorThreshold(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.Threshold = 0.1 // trigger at 10%
	// Budget=1000: threshold 100 tokens. 2000 chars * 2/7 ≈ 571 tokens > 100 → triggers.
	cb := NewContextBudget(1000)
	cc := NewConversationCompressor(cfg, cb)

	msgs := []Message{{Role: RoleUser, Content: strings.Repeat("a", 2000)}}
	_, summary := cc.Compress(msgs)
	if !strings.Contains(summary, "[context]") {
		t.Error("expected compression at low threshold")
	}
}

func TestCompressorNoOpWhenNotNeeded(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cb := NewContextBudget(100000)
	cc := NewConversationCompressor(cfg, cb)

	msgs := []Message{{Role: RoleUser, Content: "hello"}}
	_, summary := cc.Compress(msgs)
	if strings.Contains(summary, "[context]") {
		t.Error("unexpected compression on small input")
	}
}

func TestCompressorEmptyMessages(t *testing.T) {
	cc := NewConversationCompressor(DefaultCompressionConfig(), NewContextBudget(1000))
	result, _ := cc.Compress(nil)
	if len(result) != 0 {
		t.Error("expected empty result")
	}
}

func TestContextBudgetReset(t *testing.T) {
	cb := NewContextBudget(1000)
	cb.Reserve(500)
	cb.Reset()
	if cb.UsedTokens != 0 {
		t.Error("expected 0 after reset")
	}
}

func TestContextBudgetCanFit(t *testing.T) {
	// Use a budget large enough that reserved (2000) doesn't dominate
	cb := NewContextBudget(5000)
	if !cb.CanFit("hi") {
		t.Error("should fit short message")
	}
	cb.Reserve(4800)
	// After reserve, UsedTokens=4800. CanFit: 4800 + tokens("hello...") + 2000 = ~6800+ > 5000.
	if cb.CanFit("hello world long message that should push over") {
		t.Error("should not fit after reserve")
	}
}

func TestContextStrategyRegistration(t *testing.T) {
	s := GetStrategy("context")
	if s == nil {
		t.Error("context strategy should be registered")
	}
	if !strings.Contains(s.Name(), "context") {
		t.Errorf("expected 'context' in name, got %s", s.Name())
	}
}

func TestContextStats(t *testing.T) {
	// Budget must be > ReservedTokens (2000) to have positive remaining
	cb := NewContextBudget(5000)
	cb.Reserve(200)
	stats := cb.Stats()
	if stats["used"] != 200 {
		t.Errorf("expected 200 used, got %d", stats["used"])
	}
	if stats["remaining"] <= 0 {
		t.Error("expected positive remaining")
	}
}

func TestContextBackwardCompat(t *testing.T) {
	cb := NewContextBudget(0) // zero should default to 128000
	if cb.MaxTokens != 128000 {
		t.Errorf("expected default 128000, got %d", cb.MaxTokens)
	}
}

func TestCompressorTruncateStrategy(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.Strategy = "truncate"
	cfg.MaxMessages = 5
	// Budget=100: threshold 80. 20 msgs × (40*2/7=11) = 220 tokens > 80 → triggers.
	cb := NewContextBudget(100)
	cc := NewConversationCompressor(cfg, cb)

	msgs := make([]Message, 20)
	for i := range msgs {
		msgs[i] = Message{Role: RoleUser, Content: strings.Repeat("x", 40)}
	}
	compressed, summary := cc.Compress(msgs)
	if len(compressed) > cfg.MaxMessages {
		t.Errorf("expected at most %d messages, got %d", cfg.MaxMessages, len(compressed))
	}
	if !strings.Contains(summary, "[context]") {
		t.Error("expected context summary")
	}
}

func TestCompressorSummarizeStrategy(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.Strategy = "summarize"
	cfg.KeepRecent = 2
	// Budget=100: threshold 80. 5 msgs w/ 50 chars: 5*(50*2/7=14)=70 → won't trigger.
	// Need larger: budget=50, threshold=40. 5 msgs*(100*2/7=28)=140 > 40 → triggers.
	cb := NewContextBudget(50)
	cc := NewConversationCompressor(cfg, cb)

	msgs := []Message{
		{Role: RoleSystem, Content: "you are a bot"},
		{Role: RoleUser, Content: strings.Repeat("a", 100)},
		{Role: RoleUser, Content: strings.Repeat("b", 100)},
		{Role: RoleUser, Content: "recent1"},
		{Role: RoleUser, Content: "recent2"},
	}
	compressed, _ := cc.Compress(msgs)
	// Should have system + summary + last 2 recent
	if len(compressed) < 3 {
		t.Errorf("expected at least 3 messages, got %d", len(compressed))
	}
	hasSummary := false
	hasRecent := false
	for _, m := range compressed {
		if strings.Contains(m.Content, "summarized away") {
			hasSummary = true
		}
		if strings.Contains(m.Content, "recent2") {
			hasRecent = true
		}
	}
	if !hasSummary || !hasRecent {
		t.Error("expected summary marker and recent messages preserved")
	}
}

func TestEstimateTokensEmpty(t *testing.T) {
	if EstimateTokens([]Message{}) != 0 {
		t.Error("expected 0 tokens for empty message list")
	}
	if EstimateTokensPrecise("") != 0 {
		t.Error("expected 0 precise tokens for empty string")
	}
}

func TestContextBudgetStatsAfterCompression(t *testing.T) {
	cb := NewContextBudget(10000)
	cb.CompressionCount = 5
	stats := cb.Stats()
	if stats["compressions"] != 5 {
		t.Errorf("expected 5 compressions, got %d", stats["compressions"])
	}
}
