package core

import "sync"

// Message represents a single message in the transcript.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string          // for tool results
	ToolCalls  []ToolCallRequest // for assistant tool call requests
	MessageID  string
}

// ToolCallRequest is embedded in an assistant Message.
type ToolCallRequest struct {
	CallID   string
	ToolName string
	Args     string // JSON string
}

// Transcript is the authoritative message log. It is safe for concurrent use.
type Transcript struct {
	mu       sync.RWMutex
	messages []Message
}

// NewTranscript creates an empty Transcript.
func NewTranscript() *Transcript {
	return &Transcript{messages: make([]Message, 0)}
}

// Append adds one or more messages to the transcript.
func (t *Transcript) Append(msgs ...Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messages = append(t.messages, msgs...)
}

// Messages returns a snapshot copy of all messages.
func (t *Transcript) Messages() []Message {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]Message, len(t.messages))
	copy(result, t.messages)
	return result
}

// Len returns the number of messages in the transcript.
func (t *Transcript) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.messages)
}
