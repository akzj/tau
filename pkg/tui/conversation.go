package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ConversationPanel renders a scrollable message feed.
type ConversationPanel struct {
	width    int
	height   int
	messages []Message
}

// Message is a single turn in the conversation.
type Message struct {
	Role    string
	Content string
}

// NewConversationPanel creates an empty conversation panel.
func NewConversationPanel() *ConversationPanel {
	return &ConversationPanel{}
}

// AddMessage appends a message, capping at 100 entries.
func (cp *ConversationPanel) AddMessage(msg Message) {
	cp.messages = append(cp.messages, msg)
	if len(cp.messages) > 100 {
		cp.messages = cp.messages[len(cp.messages)-100:]
	}
}

// Update handles window-resize events.
func (cp *ConversationPanel) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cp.width = msg.Width
		cp.height = msg.Height - 3 // reserve for status bar + input
	}
}

// View renders the visible message window (auto-scroll to bottom).
func (cp *ConversationPanel) View() string {
	var b strings.Builder
	if cp.height < 1 {
		cp.height = 20
	}
	start := 0
	if len(cp.messages) > cp.height/2 {
		start = len(cp.messages) - cp.height/2
	}
	for i := start; i < len(cp.messages); i++ {
		m := cp.messages[i]
		role := RenderRole(m.Role)
		content := highlightPrefixes(m.Content)
		b.WriteString(role + " " + content + "\n")
	}
	return b.String()
}

// highlightPrefixes replaces literal prefixes with styled versions.
func highlightPrefixes(content string) string {
	s := content
	s = strings.ReplaceAll(s, "[reasoning]", reasoningPrefix)
	s = strings.ReplaceAll(s, "[cache]", cachePrefix)
	s = strings.ReplaceAll(s, "🔧", toolPrefix)
	return s
}