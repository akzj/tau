package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar renders agent state (idle/thinking/executing), active tools,
// cache stats, token counts, and elapsed time.
type StatusBar struct {
	status      string // "idle", "thinking", "executing"
	activeTools int
	cacheHits   int64
	cacheMisses int64
	tokens      int
	elapsed     string
}

// NewStatusBar creates a status bar in idle state.
func NewStatusBar() *StatusBar {
	return &StatusBar{status: "idle"}
}

// SetStatus updates the agent status label.
func (sb *StatusBar) SetStatus(s string) { sb.status = s }

// SetTools records the number of active tool calls.
func (sb *StatusBar) SetTools(n int) { sb.activeTools = n }

// SetCache updates cache hit/miss counters.
func (sb *StatusBar) SetCache(hits, misses int64) {
	sb.cacheHits = hits
	sb.cacheMisses = misses
}

// SetTokens records the current token count.
func (sb *StatusBar) SetTokens(n int) { sb.tokens = n }

// SetElapsed records the elapsed time string.
func (sb *StatusBar) SetElapsed(s string) { sb.elapsed = s }

// View renders the status bar as a single dim line.
func (sb *StatusBar) View() string {
	var status string
	switch sb.status {
	case "thinking":
		status = statusThinking
	case "executing":
		status = statusExecuting
	default:
		status = statusIdle
	}

	tools := ""
	if sb.activeTools > 0 {
		tools = fmt.Sprintf(" tools:%d", sb.activeTools)
	}

	cacheRate := ""
	total := sb.cacheHits + sb.cacheMisses
	if total > 0 {
		rate := float64(sb.cacheHits) / float64(total) * 100
		cacheRate = fmt.Sprintf(" cache:%.0f%%", rate)
	}

	tokens := ""
	if sb.tokens > 0 {
		tokens = fmt.Sprintf(" tokens:%d", sb.tokens)
	}

	line := lipgloss.NewStyle().Width(80).Render(
		fmt.Sprintf("%s%s%s%s %s", status, tools, cacheRate, tokens, sb.elapsed),
	)
	return dimStyle.Render(line)
}