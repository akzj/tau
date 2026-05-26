package tui

import (
	"fmt"
	"strings"
)

// View implements tea.Model.
func (m *model) View() string {
	var b strings.Builder

	// Messages area: fill from the top, clip to fit.
	maxMsgHeight := m.height - 4 // 1 status + 1 input + 2 padding
	if maxMsgHeight < 1 {
		maxMsgHeight = 1
	}

	visibleMsgs := m.messages
	if len(visibleMsgs) > maxMsgHeight {
		visibleMsgs = visibleMsgs[len(visibleMsgs)-maxMsgHeight:]
	}

	for _, msg := range visibleMsgs {
		switch msg.Role {
		case "user":
			b.WriteString(fmt.Sprintf("\n  You: %s", msg.Content))
		case "assistant":
			b.WriteString(fmt.Sprintf("\n  tau: %s", msg.Content))
		case "tool":
			b.WriteString(fmt.Sprintf("\n  %s", msg.Content))
		case "system":
			b.WriteString(fmt.Sprintf("\n  [%s]", msg.Content))
		}
	}

	// Streaming message
	if m.streaming != "" {
		b.WriteString(fmt.Sprintf("\n  tau: %s", m.streaming))
	}

	// Fill remaining space so status/input stick to bottom
	used := len(visibleMsgs)
	if m.streaming != "" {
		used++
	}
	for i := used; i < maxMsgHeight; i++ {
		b.WriteString("\n")
	}

	// Status bar
	statusLine := fmt.Sprintf("[%s]", m.status)
	if len(m.tools) > 0 {
		var active []string
		for _, ts := range m.tools {
			if ts.Status == "running" {
				active = append(active, ts.Name)
			}
		}
		if len(active) > 0 {
			statusLine += fmt.Sprintf(" tools: %s", strings.Join(active, ", "))
		}
	}
	b.WriteString(fmt.Sprintf("\n  %s", statusLine))

	// Input field
	b.WriteString(fmt.Sprintf("\n  > %s", m.input.View()))

	return b.String()
}
