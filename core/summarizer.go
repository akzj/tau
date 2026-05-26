package core

import (
	"context"
	"fmt"
	"strings"
)

// Summarize uses an LLM to compress old messages into a summary.
func (c *Conversation) Summarize(ctx context.Context, provider Provider, model ModelSpec) error {
	if len(c.Messages) <= 4 {
		return fmt.Errorf("not enough messages to summarize")
	}

	splitIdx := len(c.Messages) * 80 / 100
	if splitIdx < 2 {
		splitIdx = 2
	}
	oldMsgs := c.Messages[:splitIdx]
	recentMsgs := c.Messages[splitIdx:]

	var parts []string
	for _, m := range oldMsgs {
		line := string(m.Role) + ": " + m.Content
		if len(line) > 300 {
			line = line[:300] + "..."
		}
		parts = append(parts, line)
	}

	prompt := "Summarize this conversation in 2-3 sentences. Focus on key decisions, actions taken, and current state:\n\n" +
		strings.Join(parts, "\n")

	resp, err := provider.Complete(ctx, CompleteRequest{
		Model:    model,
		Messages: []Message{{Role: RoleUser, Content: prompt}},
	})
	if err != nil {
		c.Messages = recentMsgs
		return fmt.Errorf("summarize failed, using truncation: %w", err)
	}

	summaryMsg := Message{
		Role:    RoleSystem,
		Content: "[Conversation summary]\n" + resp.Content,
	}
	newMsgs := []Message{summaryMsg}
	newMsgs = append(newMsgs, recentMsgs...)
	c.Messages = newMsgs

	c.FitToWindow()
	return nil
}
