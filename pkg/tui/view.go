package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (m *model) View() string {
	// Delegate to session browser when active.
	if m.showBrowser && m.sessionBrowser != nil {
		return m.sessionBrowser.View()
	}

	maxMsgH := m.height - 4 // 1 status + 1 input + 2 padding
	if maxMsgH < 1 {
		maxMsgH = 1
	}

	// Panel 1: Messages area — clip to visible window, respecting scroll offset.
	visible := m.messages
	n := len(visible)
	if n > maxMsgH {
		start := n - maxMsgH - m.scrollOffset
		if start < 0 {
			start = 0
		}
		end := n - m.scrollOffset
		if end > n {
			end = n
		}
		if start >= end {
			start = end - maxMsgH
			if start < 0 {
				start = 0
			}
		}
		visible = visible[start:end]
	} else {
		m.scrollOffset = 0
	}

	var msgLines []string
	maxW := m.width - 4
	if maxW < 10 {
		maxW = 80
	}

	for _, msg := range visible {
		content := msg.Content
		// Syntax-highlight code blocks; skip truncation (ANSI codes break byte counts).
		if strings.Contains(content, "```") {
			content = highlightCodeBlocks(content)
		} else {
			content = truncateView(content, maxW)
		}
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

// --- Syntax highlighting ---

// highlightCodeBlocks finds ```lang\n...\n``` blocks and applies syntax highlighting.
func highlightCodeBlocks(text string) string {
	re := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		lang := parts[1]
		code := parts[2]
		highlighted := highlightCode(lang, code)
		return "```" + lang + "\n" + highlighted + "\n```"
	})
}

// highlightCode applies syntax coloring to a code block based on language.
func highlightCode(lang string, text string) string {
	if lang == "" {
		lang = "generic"
	}
	lines := strings.Split(text, "\n")
	var highlighted []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Comment detection (cross-language).
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "--") {
			highlighted = append(highlighted, commentStyle.Render(line))
			continue
		}
		switch lang {
		case "go", "golang":
			highlighted = append(highlighted, highlightGoLine(line))
		case "python", "py":
			highlighted = append(highlighted, highlightPythonLine(line))
		case "js", "javascript", "ts", "typescript":
			highlighted = append(highlighted, highlightJSLine(line))
		case "sh", "bash", "shell":
			highlighted = append(highlighted, highlightBashLine(line))
		default:
			highlighted = append(highlighted, highlightGenericLine(line))
		}
	}
	return strings.Join(highlighted, "\n")
}

func highlightGoLine(line string) string {
	keywords := []string{"func", "return", "if", "else", "for", "range", "switch",
		"case", "defer", "go", "select", "chan", "map", "struct", "interface",
		"type", "var", "const", "import", "package", "break", "continue",
		"fallthrough", "default"}
	types := []string{"string", "int", "bool", "error", "byte", "rune",
		"float64", "float32", "int64", "int32", "uint64", "uint32", "uint",
		"uintptr", "complex64", "complex128"}
	result := colorStrings(line)
	result = colorNumbers(result)
	for _, kw := range keywords {
		result = strings.ReplaceAll(result, kw, keywordStyle.Render(kw))
	}
	for _, t := range types {
		result = strings.ReplaceAll(result, t, typeStyle.Render(t))
	}
	return result
}

func highlightPythonLine(line string) string {
	keywords := []string{"def", "return", "if", "elif", "else", "for", "while",
		"try", "except", "finally", "with", "as", "import", "from", "class",
		"pass", "break", "continue", "yield", "raise", "assert", "lambda",
		"and", "or", "not", "in", "is", "None", "True", "False"}
	result := colorStrings(line)
	for _, kw := range keywords {
		result = strings.ReplaceAll(result, kw, keywordStyle.Render(kw))
	}
	return result
}

func highlightJSLine(line string) string {
	keywords := []string{"function", "return", "if", "else", "for", "while",
		"switch", "case", "break", "continue", "try", "catch", "finally",
		"throw", "new", "delete", "typeof", "instanceof", "void", "this",
		"class", "extends", "super", "import", "export", "default", "from",
		"const", "let", "var", "async", "await", "yield", "of", "in"}
	result := colorStrings(line)
	for _, kw := range keywords {
		result = strings.ReplaceAll(result, kw, keywordStyle.Render(kw))
	}
	return result
}

func highlightBashLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return commentStyle.Render(line)
	}
	return line
}

func highlightGenericLine(line string) string {
	return colorStrings(line)
}

func colorStrings(s string) string {
	strPattern := `"(?:[^"\\]|\\.)*"|` + "`[^`]*`"
	re := regexp.MustCompile(strPattern)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		return stringStyle.Render(match)
	})
}

func colorNumbers(s string) string {
	re := regexp.MustCompile(`\b\d+\.?\d*\b`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		return numberStyle.Render(match)
	})
}

func truncateView(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}