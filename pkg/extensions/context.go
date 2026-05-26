package extensions

import (
	"fmt"
	"sync"
)

// ExtensionContext is passed to event handlers and tool/command handlers.
// It manages session lifecycle and enforces stale-context safety (pi pattern).
type ExtensionContext struct {
	mu     sync.Mutex
	active bool

	SessionID string

	// Callbacks — wired by the runtime host.
	sendMessage   func(text string)
	newSession    func() (*ExtensionContext, error)
	fork          func() (*ExtensionContext, error)
	switchSession func(id string) (*ExtensionContext, error)
}

// NewContext creates an active ExtensionContext.
func NewContext(sessionID string) *ExtensionContext {
	return &ExtensionContext{
		active:    true,
		SessionID: sessionID,
	}
}

// SendMessage sends a message to the session (e.g., for TUI display).
func (ctx *ExtensionContext) SendMessage(text string) error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if !ctx.active {
		return fmt.Errorf("stale context: this extension context is stale after session replacement")
	}
	if ctx.sendMessage != nil {
		ctx.sendMessage(text)
	}
	return nil
}

// NewSession creates a new session and returns its context.
// The old context becomes stale.
func (ctx *ExtensionContext) NewSession() (*ExtensionContext, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if !ctx.active {
		return nil, fmt.Errorf("stale context")
	}
	if ctx.newSession != nil {
		newCtx, err := ctx.newSession()
		if err != nil {
			return nil, err
		}
		ctx.active = false
		return newCtx, nil
	}
	return nil, fmt.Errorf("NewSession not bound")
}

// Fork creates a fork of the current session.
// The old context becomes stale.
func (ctx *ExtensionContext) Fork() (*ExtensionContext, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if !ctx.active {
		return nil, fmt.Errorf("stale context")
	}
	if ctx.fork != nil {
		newCtx, err := ctx.fork()
		if err != nil {
			return nil, err
		}
		ctx.active = false
		return newCtx, nil
	}
	return nil, fmt.Errorf("Fork not bound")
}

// SwitchSession switches to another session by ID.
// The old context becomes stale.
func (ctx *ExtensionContext) SwitchSession(id string) (*ExtensionContext, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if !ctx.active {
		return nil, fmt.Errorf("stale context")
	}
	if ctx.switchSession != nil {
		newCtx, err := ctx.switchSession(id)
		if err != nil {
			return nil, err
		}
		ctx.active = false
		return newCtx, nil
	}
	return nil, fmt.Errorf("SwitchSession not bound")
}

// IsActive returns whether this context is still valid.
func (ctx *ExtensionContext) IsActive() bool {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	return ctx.active
}

// SetCallbacks wires the context actions.
func (ctx *ExtensionContext) SetCallbacks(
	sendMsg func(text string),
	newSess func() (*ExtensionContext, error),
	forkFn func() (*ExtensionContext, error),
	switchFn func(id string) (*ExtensionContext, error),
) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.sendMessage = sendMsg
	ctx.newSession = newSess
	ctx.fork = forkFn
	ctx.switchSession = switchFn
}
