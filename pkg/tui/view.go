package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color theme — dark terminal palette.
var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#4fc3f7")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#81c784"))
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#a5d6a7")).Faint(true)
	systemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#90a4ae")).Italic(true)
	thinkingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Italic(true)
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

	view := strings.Join(msgLines, "\n") + "\n" + statusBar + "\n" + inputLine

	// Overlay file tree sidebar when toggled.
	if m.showFiles {
		view = m.overlayFileTree(view)
	}

	return view
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
	case "thinking":
		return "  💭"
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
	case "thinking":
		return thinkingStyle
	default:
		return lipgloss.NewStyle()
	}
}

// overlayFileTree renders the file tree sidebar on the right side of the main view.
func (m *model) overlayFileTree(mainView string) string {
	tree := m.renderFileTree()
	if tree == "" {
		return mainView
	}

	panelW := 24 // right panel width
	mainLines := strings.Split(mainView, "\n")
	treeLines := strings.Split(tree, "\n")

	var result []string
	maxH := len(mainLines)
	if len(treeLines) > maxH {
		maxH = len(treeLines)
	}

	for i := 0; i < maxH; i++ {
		left := ""
		if i < len(mainLines) {
			left = mainLines[i]
		}
		right := ""
		if i < len(treeLines) {
			right = treeLines[i]
		}
		result = append(result, padRight(left, m.width-panelW)+right)
	}
	return strings.Join(result, "\n")
}

// renderFileTree builds the file tree panel string.
func (m *model) renderFileTree() string {
	files := m.fileTree.List()
	if len(files) == 0 {
		return ""
	}

	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("┌ Files ──────────┐\n")

	// Show last 8 files (fit in panel).
	visible := paths
	if len(visible) > 8 {
		visible = visible[len(visible)-8:]
	}

	for _, p := range visible {
		status := files[p]
		icon := " "
		switch status {
		case FileCreated:
			icon = "●"
		case FileModified:
			icon = "○"
		case FileRead:
			icon = "·"
		}
		display := icon + " " + shortenPath(p, 17)
		b.WriteString(display + "\n")
	}

	// Fill remaining lines.
	for i := len(visible); i < 8; i++ {
		b.WriteString("\n")
	}
	b.WriteString("└─────────────────┘")
	return b.String()
}

// padRight pads s to length n with spaces.
func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

// shortenPath truncates a path to fit max chars, keeping the tail.
func shortenPath(p string, max int) string {
	if len(p) <= max {
		return p
	}
	return "…" + p[len(p)-max+3:]
}

func truncateView(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}