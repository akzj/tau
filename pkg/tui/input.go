package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// InputPanel manages the user input line with history and tab completion.
type InputPanel struct {
	text      strings.Builder
	history   []string
	histIdx   int
	toolNames []string // for tab completion
	focused   bool
}

// NewInputPanel creates an input panel with optional tool-name completions.
func NewInputPanel(toolNames []string) *InputPanel {
	if toolNames == nil {
		toolNames = []string{}
	}
	return &InputPanel{
		toolNames: toolNames,
		focused:   true,
	}
}

// Update processes key events for the input panel.
func (ip *InputPanel) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "backspace":
			s := ip.text.String()
			if len(s) > 0 {
				ip.text.Reset()
				ip.text.WriteString(s[:len(s)-1])
			}
		case "up":
			if ip.histIdx > 0 {
				ip.histIdx--
				ip.text.Reset()
				ip.text.WriteString(ip.history[ip.histIdx])
			}
		case "down":
			if ip.histIdx < len(ip.history)-1 {
				ip.histIdx++
				ip.text.Reset()
				ip.text.WriteString(ip.history[ip.histIdx])
			} else if ip.histIdx == len(ip.history)-1 {
				ip.histIdx++
				ip.text.Reset()
			}
		case "tab":
			ip.doComplete()
		default:
			// Printable characters only.
			if len(msg.Runes) == 1 && msg.Runes[0] >= 32 {
				ip.text.WriteRune(msg.Runes[0])
			}
		}
	}
}

// Submit returns the current input text, adds it to history, and clears the buffer.
// Returns empty string if input is blank.
func (ip *InputPanel) Submit() string {
	text := strings.TrimSpace(ip.text.String())
	if text == "" {
		return ""
	}
	ip.history = append(ip.history, text)
	ip.histIdx = len(ip.history)
	ip.text.Reset()
	return text
}

// SetToolNames updates the tab-completion list.
func (ip *InputPanel) SetToolNames(names []string) {
	ip.toolNames = names
}

// doComplete tries to match the current input against known tool names.
func (ip *InputPanel) doComplete() {
	partial := ip.text.String()
	for _, name := range ip.toolNames {
		if strings.HasPrefix(name, partial) {
			ip.text.Reset()
			ip.text.WriteString(name)
			return
		}
	}
}

// View renders the input prompt.
func (ip *InputPanel) View() string {
	prompt := "> "
	if !ip.focused {
		prompt = "  "
	}
	return prompt + ip.text.String() + " ▌"
}