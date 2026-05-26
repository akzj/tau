package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color theme — dark terminal palette.
var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#4fc3f7")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#81c784"))
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#a5d6a7")).Faint(true)
	systemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#90a4ae")).Italic(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef5350"))
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#78909c")).Background(lipgloss.Color("#263238"))
	streamingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#81c784")).Faint(true)
)

// View implements tea.Model.
func (m *model) View() string {
	maxMsgH := m.height - 4 // 1 status + 1 input + 2 padding
	if maxMsgH < 1 {
		maxMsgH = 1
	}

	// Panel 1: Messages area — clip to visible window.
	visible := m.messages
	if len(visible) > maxMsgH {
		visible = visible[len(visible)-maxMsgH:]
	}

	var msgLines []string
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 80
	}

	for _, msg := range visible {
		content := truncateView(msg.Content, maxW)
		prefix := prefixFor(msg.Role)
		style := styleFor(msg.Role)
		msgLines = append(msgLines, style.Render(prefix+" "+content))
	}

	// Streaming output with cursor.
	if m.streaming != "" {
		stream := truncateView(m.streaming, maxW)
		msgLines = append(msgLines, streamingStyle.Render("tau: "+stream+"▌"))
	}

	// Fill remaining space so status/input stick to bottom.
	used := len(msgLines)
	for i := used; i < maxMsgH; i++ {
		msgLines = append(msgLines, "")
	}

	// Panel 2: Status bar — left: status+tools, right: msg/turn counts.
	statusLeft := fmt.Sprintf("[%s]", m.status)
	if len(m.tools) > 0 {
		var running []string
		for _, t := range m.tools {
			if t.Status == "running" {
				running = append(running, t.Name)
			}
		}
		if len(running) > 0 {
			statusLeft += " 🔧 " + strings.Join(running, ",")
		}
	}
	if m.err != nil {
		statusLeft += " ⚠"
	}

	statusRight := fmt.Sprintf("msgs: %d | turns: %d", len(m.messages), m.turnCount)
	gap := m.width - len(statusLeft) - len(statusRight) - 2
	if gap < 1 {
		gap = 1
	}
	statusBar := statusStyle.Render(statusLeft + strings.Repeat(" ", gap) + statusRight)

	// Panel 3: Input field.
	inputLine := "> " + m.input.View()

	return strings.Join(msgLines, "\n") + "\n" + statusBar + "\n" + inputLine
}

func prefixFor(role string) string {
	switch role {
	case "user":
		return "You:"
	case "assistant":
		return "tau:"
	case "tool":
		return "  🔧"
	case "system":
		return "  ⚙"
	default:
		return "  ?"
	}
}

func styleFor(role string) lipgloss.Style {
	switch role {
	case "user":
		return userStyle
	case "assistant":
		return assistantStyle
	case "tool":
		return toolStyle
	case "system":
		return systemStyle
	default:
		return lipgloss.NewStyle()
	}
}

func truncateView(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}