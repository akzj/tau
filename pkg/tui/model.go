// Package tui provides a Bubble Tea terminal UI for the tau coding agent.
// Architecture: Model/Update/View with a goroutine bridge that runs the
// agent turn loop (Prompt → events → Continue → …) and pumps AgentEvents
// into the TUI update cycle.
package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
)

// line is a single rendered message line.
type line struct {
	Role    string // "user", "assistant", "tool", "system"
	Content string
	Details map[string]any // structured tool result details
}

// toolState tracks an in-flight tool call.
type toolState struct {
	Name   string
	Status string // "running", "done"
	Result string
}

// model is the Bubble Tea model for the TUI.
type model struct {
	session       *coding.CodingSession
	loop          core.Loop
	messages      []line
	streaming     string
	tools         map[string]toolState
	input         textinput.Model
	status        string
	width         int
	height        int
	agentChan     chan any // bridge: agent goroutine → TUI (core.AgentEvent | turnCompleteMsg)
	err           error
	initialPrompt string // if non-empty, auto-submit on start
	turnCount     int    // number of completed turns
	activeTools   []string // current active tool set
}

// turnCompleteMsg signals the turn loop finished.
type turnCompleteMsg struct{ err error }

// eventMsg wraps an AgentEvent for the Update loop.
type eventMsg struct {
	event core.AgentEvent
}

// NewModel creates a new TUI model.
// initialPrompt is optional — if non-empty, it will be auto-submitted on start.
func NewModel(sess *coding.CodingSession, loop core.Loop, initialPrompt string) *model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	status := "idle"
	if initialPrompt != "" {
		status = "streaming"
	}

	return &model{
		session:       sess,
		loop:          loop,
		messages:      make([]line, 0),
		tools:         make(map[string]toolState),
		input:         ti,
		status:        status,
		agentChan:     make(chan any, 64),
		initialPrompt: initialPrompt,
	}
}

// Init implements tea.Model.
func (m *model) Init() tea.Cmd {
	if m.initialPrompt != "" {
		prompt := m.initialPrompt
		m.initialPrompt = ""
		m.messages = append(m.messages, line{Role: "user", Content: prompt})
		go m.runTurnLoop(prompt)
	}
	return listenEvents(m.agentChan)
}

// listenEvents returns a Cmd that reads the next value from the bridge channel.
func listenEvents(ch <-chan any) tea.Cmd {
	return func() tea.Msg {
		v, ok := <-ch
		if !ok {
			return nil
		}
		switch v := v.(type) {
		case core.AgentEvent:
			return eventMsg{event: v}
		case turnCompleteMsg:
			return v
		default:
			return nil
		}
	}
}
