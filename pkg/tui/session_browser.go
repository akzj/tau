package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/akzj/tau/pkg/persist"
)

// sessionBrowser is a sub-model for the session management panel.
type sessionBrowser struct {
	sessions  []persist.SessionInfo
	cursor    int
	deleteIdx int    // -1 = none, >=0 = confirming delete
	width     int
	height    int
	activeID  string // currently loaded session ID
}

var (
	sbActiveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4fc3f7")).Bold(true)
	sbInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	sbSelectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("#1f6feb")).Foreground(lipgloss.Color("#ffffff"))
	sbConfirmStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef5350")).Bold(true)
	sbTitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1d9")).Bold(true).Padding(0, 1)
	sbHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#484f58")).Padding(0, 1)
)

func newSessionBrowser(activeID string) *sessionBrowser {
	sessions, _ := persist.List()
	return &sessionBrowser{
		sessions:  sessions,
		cursor:    0,
		deleteIdx: -1,
		activeID:  activeID,
	}
}

func (sb *sessionBrowser) Init() tea.Cmd { return nil }

func (sb *sessionBrowser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if sb.deleteIdx >= 0 {
			// Confirm delete mode
			switch msg.String() {
			case "y", "Y":
				id := sb.sessions[sb.deleteIdx].ID
				persist.Remove(id)
				sb.sessions, _ = persist.List()
				sb.deleteIdx = -1
				if sb.cursor >= len(sb.sessions) && sb.cursor > 0 {
					sb.cursor = len(sb.sessions) - 1
				}
				return sb, nil
			case "n", "N", "esc":
				sb.deleteIdx = -1
				return sb, nil
			}
			return sb, nil
		}

		switch msg.String() {
		case "up", "k":
			if sb.cursor > 0 {
				sb.cursor--
			}
		case "down", "j":
			if sb.cursor < len(sb.sessions)-1 {
				sb.cursor++
			}
		case "enter":
			if len(sb.sessions) > 0 {
				return sb, loadSessionCmd(sb.sessions[sb.cursor].ID)
			}
		case "ctrl+d":
			if len(sb.sessions) > 0 && sb.cursor < len(sb.sessions) {
				sb.deleteIdx = sb.cursor
			}
		case "ctrl+s":
			// Quick-save current session
			id := persist.NewID()
			return sb, saveSessionCmd(id)
		case "ctrl+n":
			return sb, newSessionCmd()
		case "esc", "ctrl+b":
			return sb, closeBrowserCmd()
		}
	case tea.WindowSizeMsg:
		sb.width = msg.Width
		sb.height = msg.Height
	}
	return sb, nil
}

// --- Browser message types & cmds ---

type loadSessionMsg struct{ id string }
type closeBrowserMsg struct{}
type newSessionMsg struct{}
type saveSessionMsg struct{ id string }

func loadSessionCmd(id string) tea.Cmd {
	return func() tea.Msg { return loadSessionMsg{id: id} }
}
func closeBrowserCmd() tea.Cmd {
	return func() tea.Msg { return closeBrowserMsg{} }
}
func newSessionCmd() tea.Cmd {
	return func() tea.Msg { return newSessionMsg{} }
}
func saveSessionCmd(id string) tea.Cmd {
	return func() tea.Msg { return saveSessionMsg{id: id} }
}

func (sb *sessionBrowser) View() string {
	var b strings.Builder

	// Title
	b.WriteString(sbTitleStyle.Render("Sessions"))
	b.WriteString("\n")

	if sb.deleteIdx >= 0 {
		s := sb.sessions[sb.deleteIdx]
		b.WriteString(sbConfirmStyle.Render(fmt.Sprintf("Delete %s? (y/n)", s.ID)))
		b.WriteString("\n")
		return b.String()
	}

	// Session list — clip to available height.
	maxItems := sb.height - 5
	if maxItems < 1 {
		maxItems = 1
	}
	start := sb.cursor - maxItems/2
	if start < 0 {
		start = 0
	}
	end := start + maxItems
	if end > len(sb.sessions) {
		end = len(sb.sessions)
	}

	for i := start; i < end; i++ {
		s := sb.sessions[i]
		prefix := "○"
		if s.ID == sb.activeID {
			prefix = "●"
		}
		line := fmt.Sprintf(" %s %-20s %3d msgs", prefix, s.ID, s.MsgCount)
		if i == sb.cursor {
			line = sbSelectedStyle.Render(line)
		} else if s.ID == sb.activeID {
			line = sbActiveStyle.Render(line)
		} else {
			line = sbInactiveStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Fill remaining lines.
	for i := end - start; i < maxItems; i++ {
		b.WriteString("\n")
	}

	// Help bar
	b.WriteString("\n")
	b.WriteString(sbHelpStyle.Render("↑↓:nav Enter:load Ctrl+D:del Ctrl+S:save Ctrl+N:new Esc:back"))

	return b.String()
}