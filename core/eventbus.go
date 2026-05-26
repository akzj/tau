package core

import "sync"

// EventType identifiers.
type EventType string

const (
	EvtTurnStart    EventType = "turn:start"
	EvtTurnEnd      EventType = "turn:end"
	EvtMessageStart EventType = "message:start"
	EvtMessageEnd   EventType = "message:end"
	EvtToolCallStart EventType = "tool:start"
	EvtToolCallEnd  EventType = "tool:end"
	EvtCompaction   EventType = "compaction"
	EvtError        EventType = "error"
	EvtSessionStart EventType = "session:start"
	EvtSessionEnd   EventType = "session:end"
)

// Event is an emitted event with type and payload.
type Event struct {
	Type    EventType
	Payload any
}

// EventBus is a thread-safe event subscription system.
type EventBus struct {
	mu          sync.RWMutex
	subscribers []chan Event
}

// NewEventBus creates an EventBus.
func NewEventBus() *EventBus {
	return &EventBus{}
}

// Subscribe returns a channel that receives all events. Call cancel() to detach.
func (eb *EventBus) Subscribe() (<-chan Event, func()) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan Event, 64)
	eb.subscribers = append(eb.subscribers, ch)
	cancel := func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		for i, sub := range eb.subscribers {
			if sub == ch {
				eb.subscribers = append(eb.subscribers[:i], eb.subscribers[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return ch, cancel
}

// Emit sends an event to all subscribers (non-blocking).
func (eb *EventBus) Emit(evt Event) {
	eb.mu.RLock()
	subs := eb.subscribers
	eb.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}
