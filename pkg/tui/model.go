// Package tui provides a Bubble Tea terminal UI for the tau coding agent.
// Architecture: Model/Update/View with a goroutine bridge that runs the
// agent turn loop (Prompt → events → Continue → …) and pumps AgentEvents
// into the TUI update cycle.
package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// FileStatus tracks the state of a file in the workspace.
type FileStatus string

const (
	FileRead     FileStatus = "read"
	FileCreated  FileStatus = "created"
	FileModified FileStatus = "modified"
)

// FileTree tracks file operations during a session.
type FileTree struct {
	files map[string]FileStatus // path → status
}

// NewFileTree creates an empty FileTree.
func NewFileTree() *FileTree {
	return &FileTree{files: make(map[string]FileStatus)}
}

// Mark records a file operation. Modified overwrites read; created overwrites modified.
func (ft *FileTree) Mark(path string, status FileStatus) {
	if existing, ok := ft.files[path]; ok {
		if existing == FileCreated {
			return
		}
		if existing == FileModified && status == FileRead {
			return
		}
	}
	ft.files[path] = status
}

// List returns a copy of the file tree.
func (ft *FileTree) List() map[string]FileStatus {
	out := make(map[string]FileStatus, len(ft.files))
	for k, v := range ft.files {
		out[k] = v
	}
	return out
}

// toolState tracks an in-flight tool call.
type toolState struct {
	Name   string
	Status string          // "running", "done"
	Result string
	Args   json.RawMessage // captured from ToolCallStart
}

// model is the Bubble Tea model for the TUI.
type model struct {
	session       *coding.CodingSession
	loop          core.Loop
	messages      []line
	streaming     string
	thinkingBuf   strings.Builder
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
	fileTree      *FileTree // file operations in this session
	showFiles     bool      // Ctrl+T toggle: show file tree sidebar
	providers     []string  // available provider IDs
	providerIdx   int       // current provider index
	models        []string  // available model IDs for current provider
	modelIdx      int       // current model index
	scrollOffset  int       // mouse wheel scroll offset (lines scrolled up)
	sessionBrowser *sessionBrowser // session browser sub-model (lazy init)
	showBrowser    bool            // true = session browser active
	streamUI       chan core.StreamEvent // streaming UI channel (nil = non-streaming)
}

// turnCompleteMsg signals the turn loop finished.
type turnCompleteMsg struct{ err error }

// eventMsg wraps an AgentEvent for the Update loop.
type eventMsg struct {
	event core.AgentEvent
}

// NewModel creates a new TUI model.
// initialPrompt is optional — if non-empty, it will be auto-submitted on start.
// streamUI is the streaming channel (nil = non-streaming mode).
func NewModel(sess *coding.CodingSession, loop core.Loop, initialPrompt string, streamUI chan core.StreamEvent) *model {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()

	status := "idle"
	if initialPrompt != "" {
		status = "streaming"
	}

	m := &model{
		session:       sess,
		loop:          loop,
		messages:      make([]line, 0),
		tools:         make(map[string]toolState),
		input:         ti,
		status:        status,
		agentChan:     make(chan any, 64),
		initialPrompt: initialPrompt,
		fileTree:      NewFileTree(),
		showFiles:     false,
		providers:     []string{"openai", "anthropic", "google", "azure", "mistral", "bedrock", "vertex"},
		providerIdx:   0,
		models:        []string{"gpt-5.4", "gpt-4o", "gpt-4o-mini", "gpt-4", "o1", "o1-mini"},
		modelIdx:      0,
		scrollOffset:  0,
		sessionBrowser: nil,
		showBrowser:    false,
		streamUI:       streamUI,
	}

	// Start streaming consumer if StreamUI channel is provided.
	if streamUI != nil {
		go m.bridgeStreamUI()
	}

	return m
}

// subscribeEvents wires the model to the session event bus.
func (m *model) subscribeEvents() {
	if m.session.Session.EventBus == nil {
		return
	}
	ch, _ := m.session.Session.EventBus.Subscribe()
	go func() {
		for evt := range ch {
			switch evt.Type {
			case core.EvtTurnStart:
				m.status = "streaming"
			case core.EvtTurnEnd:
				m.status = "idle"
			case core.EvtError:
				m.status = "error"
			}
		}
	}()
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

// bridgeStreamUI consumes the StreamUI channel and injects synthetic
// MessageDelta/MessageEnd events into the agent channel for realtime TUI rendering.
func (m *model) bridgeStreamUI() {
	for ev := range m.streamUI {
		switch ev.Type {
		case core.StreamDelta:
			if ev.Content != "" {
				m.agentChan <- eventMsg{event: core.MessageDelta{
					Timestamp_:   time.Now(),
					ContentDelta: ev.Content,
				}}
			}
		case core.StreamDone:
			m.agentChan <- eventMsg{event: core.MessageEnd{
				Timestamp_: time.Now(),
			}}
		case core.StreamError:
			m.agentChan <- turnCompleteMsg{err: ev.Err}
		}
	}
}
// ── AppModel — composite Bubble Tea model ─────────────────────────────

// AppModel composes ConversationPanel, InputPanel, StatusBar, and
// EventSubscriber into a single Bubble Tea application model.
type AppModel struct {
	conversation *ConversationPanel
	input        *InputPanel
	status       *StatusBar
	browser      *sessionBrowser
	subscriber   *EventSubscriber
	width        int
	height       int
}

// NewAppModel creates a composite AppModel with all panels wired.
func NewAppModel(toolNames []string) *AppModel {
	return &AppModel{
		conversation: NewConversationPanel(),
		input:        NewInputPanel(toolNames),
		status:       NewStatusBar(),
		subscriber:   NewEventSubscriber(),
	}
}

// Init implements tea.Model.
func (m *AppModel) Init() tea.Cmd {
	return listenEvents(m.subscriber.Chan())
}

// Update implements tea.Model.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.conversation.Update(msg)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.input.Update(msg)
		if msg.String() == "ctrl+d" {
			text := m.input.Submit()
			if text != "" {
				m.conversation.AddMessage(Message{Role: "user", Content: text})
				m.status.SetStatus("thinking")
			}
		}

	case MessageEvent:
		m.conversation.AddMessage(Message{Role: msg.Role, Content: msg.Content})

	case ToolCallEvent:
		m.status.SetTools(m.status.activeTools + 1)

	case CacheEvent:
		if msg.Hit {
			m.status.cacheHits++
		} else {
			m.status.cacheMisses++
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m *AppModel) View() string {
	conv := m.conversation.View()
	input := m.input.View()
	status := m.status.View()

	// Fill conversation area to push input + status to bottom.
	return fmt.Sprintf("%s\n%s\n%s", conv, input, status)
}