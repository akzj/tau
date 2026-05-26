package core

import (
	"testing"
)

func TestConversationAdd(t *testing.T) {
	c := NewConversation(100000, StrategySliding)
	c.Add(Message{Role: RoleUser, Content: "hello"})
	c.Add(Message{Role: RoleAssistant, Content: "hi"})
	if c.Len() != 2 {
		t.Errorf("expected 2 messages, got %d", c.Len())
	}
}

func TestConversationFitToWindowSliding(t *testing.T) {
	c := NewConversation(5, StrategySliding)
	msgs := []Message{
		{Role: RoleUser, Content: "this is a very long message that should be trimmed"},
		{Role: RoleAssistant, Content: "short reply"},
		{Role: RoleUser, Content: "another message"},
	}
	for _, m := range msgs {
		c.Add(m)
	}
	if c.Len() >= 3 {
		t.Errorf("expected trimming, got %d messages", c.Len())
	}
}

func TestConversationFitToWindowTruncate(t *testing.T) {
	c := NewConversation(5, StrategyTruncate)
	msgs := []Message{
		{Role: RoleUser, Content: "keep me first"},
		{Role: RoleAssistant, Content: "also keep"},
		{Role: RoleUser, Content: "drop me please"},
	}
	for _, m := range msgs {
		c.Add(m)
	}
	if c.Len() >= 3 {
		t.Errorf("expected trimming, got %d messages", c.Len())
	}
	if c.Messages[0].Content != "keep me first" {
		t.Errorf("expected 'keep me first', got %q", c.Messages[0].Content)
	}
}

func TestConversationToMessages(t *testing.T) {
	c := NewConversation(100000, StrategySliding)
	c.Add(Message{Role: RoleUser, Content: "test"})
	result := c.ToMessages()
	if len(result) != 1 {
		t.Errorf("expected 1 message, got %d", len(result))
	}
	result[0].Content = "modified"
	if c.Messages[0].Content == "modified" {
		t.Error("ToMessages should return a copy")
	}
}

func TestConversationTokenCount(t *testing.T) {
	c := NewConversation(100000, StrategySliding)
	c.Add(Message{Role: RoleUser, Content: "hello world"})
	if c.TokenCount() == 0 {
		t.Error("expected non-zero token count")
	}
}
