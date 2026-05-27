package tui

import "github.com/charmbracelet/lipgloss"

// ── Consolidated TUI styles ────────────────────────────────────────────
// Moved from view.go and extended with Phase 1 additions.

// Role colors.
var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#4fc3f7")).Bold(true)  // blue
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#81c784"))             // green
	toolStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd54f"))             // yellow (was #a5d6a7)
	systemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#90a4ae")).Italic(true)
	thinkingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Italic(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef5350")).Bold(true)  // red (+Bold)
	reasoningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ce93d8"))             // purple
	cacheStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4dd0e1"))             // cyan
)

// Status indicators (rendered once, reused).
var (
	statusIdle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e")).Render("● idle")
	statusThinking  = lipgloss.NewStyle().Foreground(lipgloss.Color("#81c784")).Render("● thinking")
	statusExecuting = lipgloss.NewStyle().Foreground(lipgloss.Color("#4fc3f7")).Render("● executing")
)

// Utility styles.
var (
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#484f58"))
	borderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#30363d"))
)

// Prefix templates (rendered once, reused).
var (
	reasoningPrefix = lipgloss.NewStyle().Foreground(lipgloss.Color("#ce93d8")).Render("[reasoning]")
	cachePrefix     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4dd0e1")).Render("[cache]")
	toolPrefix      = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd54f")).Render("🔧")
	errorPrefix     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef5350")).Render("[error]")
)

// Streaming / status-bar / syntax styles (retained from view.go).
var (
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#78909c")).Background(lipgloss.Color("#263238"))
	streamingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#81c784")).Faint(true)
	keywordStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4fc3f7")).Bold(true)
	stringStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a5d6a7"))
	commentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#616161")).Italic(true)
	numberStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffcc80"))
	typeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#82b1ff"))
)

// ── Helpers ────────────────────────────────────────────────────────────

// RenderRole returns a styled label for the given role.
func RenderRole(role string) string {
	switch role {
	case "user":
		return userStyle.Render("You")
	case "assistant":
		return assistantStyle.Render("tau")
	case "tool":
		return toolStyle.Render("🔧")
	case "error":
		return errorStyle.Render("❌")
	case "system":
		return dimStyle.Render("system")
	default:
		return role
	}
}