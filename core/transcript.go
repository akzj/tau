package core

import "sync"

// Message represents a single message in the transcript.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string            // for tool results
	ToolCalls  []ToolCallRequest // for assistant tool call requests
	MessageID  string
}

// ToolCallRequest is embedded in an assistant Message.
type ToolCallRequest struct {
	CallID   string
	ToolName string
	Args     string // JSON string
}

// Position is a stable cursor into the Transcript.
type Position int64

// subscriber tracks one active subscription.
type subscriber struct {
	ch     chan Message
	cursor Position
}

// Transcript is the authoritative message log. It is safe for concurrent use.
type Transcript struct {
	mu          sync.RWMutex
	messages    []Message
	subscribers map[int]*subscriber
	nextSubID   int
}

// NewTranscript creates an empty Transcript.
func NewTranscript() *Transcript {
	return &Transcript{
		messages:    make([]Message, 0),
		subscribers: make(map[int]*subscriber),
	}
}

// Append adds one or more messages to the transcript and notifies subscribers.
func (t *Transcript) Append(msgs ...Message) {
	t.mu.Lock()
	t.messages = append(t.messages, msgs...)
	newLen := Position(len(t.messages))

	// Collect updates for each subscriber before releasing lock.
	type subUpdate struct {
		ch   chan Message
		msgs []Message
	}
	var updates []subUpdate
	for _, sub := range t.subscribers {
		if sub.cursor < newLen {
			msgs := make([]Message, int(newLen)-int(sub.cursor))
			copy(msgs, t.messages[int(sub.cursor):int(newLen)])
			updates = append(updates, subUpdate{ch: sub.ch, msgs: msgs})
			sub.cursor = newLen
		}
	}
	t.mu.Unlock()

	// Send outside lock to avoid deadlock; non-blocking.
	for _, u := range updates {
		for _, m := range u.msgs {
			select {
			case u.ch <- m:
			default:
				// drop if subscriber is slow
			}
		}
	}
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

// Slice returns messages in the range [from, to). Positions are message indices.
func (t *Transcript) Slice(from, to Position) []Message {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := Position(len(t.messages))
	if from < 0 {
		from = 0
	}
	if to < 0 || to > n {
		to = n
	}
	if from > to {
		from = to
	}
	result := make([]Message, int(to-from))
	copy(result, t.messages[int(from):int(to)])
	return result
}

// Subscribe returns a channel of new messages from cursor forward.
// Multiple subscribers see the same events. Call cancel() to detach.
func (t *Transcript) Subscribe(cursor Position) (<-chan Message, func()) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := t.nextSubID
	t.nextSubID++
	ch := make(chan Message, 64)
	t.subscribers[id] = &subscriber{ch: ch, cursor: cursor}

	cancel := func() {
		t.mu.Lock()
		delete(t.subscribers, id)
		t.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}

// Compact replaces messages before firstKept with a single system summary message.
// Used by the compaction system (G4 Path A).
func (t *Transcript) Compact(summary string, firstKept Position) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if int(firstKept) >= len(t.messages) {
		return
	}

	summaryMsg := Message{
		Role:    RoleSystem,
		Content: "[Compacted history]\n" + summary,
	}

	kept := make([]Message, len(t.messages)-int(firstKept))
	copy(kept, t.messages[int(firstKept):])
	t.messages = append([]Message{summaryMsg}, kept...)
}
