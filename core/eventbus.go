package core

import "sync"

// EventType identifiers.
type EventType string

const (
	// EvtTurnStart fires when a new turn begins.
	EvtTurnStart EventType = "turn:start"
	// EvtTurnEnd fires when a turn completes.
	EvtTurnEnd EventType = "turn:end"
	// EvtMessageStart fires when an assistant message begins streaming.
	EvtMessageStart EventType = "message:start"
	// EvtMessageEnd fires when an assistant message finishes streaming.
	EvtMessageEnd EventType = "message:end"
	// EvtToolCallStart fires when a tool call is requested.
	EvtToolCallStart EventType = "tool:start"
	// EvtToolCallEnd fires when a tool call completes.
	EvtToolCallEnd EventType = "tool:end"
	// EvtCompaction fires after transcript compaction.
	EvtCompaction EventType = "compaction"
	// EvtError fires on provider or tool errors.
	EvtError EventType = "error"
	// EvtSessionStart fires when a new session is created.
	EvtSessionStart EventType = "session:start"
	// EvtSessionEnd fires when a session is closed.
	EvtSessionEnd EventType = "session:end"

	// EvtSessionCompact fires when compaction runs.
	EvtSessionCompact EventType = "session:compact"
	// EvtSessionFork fires when a session is forked.
	EvtSessionFork EventType = "session:fork"
	// EvtSessionSave fires when a session is persisted.
	EvtSessionSave EventType = "session:save"
	// EvtSessionLoad fires when a session is loaded from disk.
	EvtSessionLoad EventType = "session:load"

	// EvtToolPrepare fires before a tool's Prepare phase.
	EvtToolPrepare EventType = "tool:prepare"
	// EvtToolFinalize fires after a tool's Finalize phase.
	EvtToolFinalize EventType = "tool:finalize"

	// EvtProviderRequest fires before a provider Stream call.
	EvtProviderRequest EventType = "provider:request"
	// EvtProviderResponse fires after a provider Stream call returns.
	EvtProviderResponse EventType = "provider:response"

	// EvtSteerInjected fires when a steer instruction is consumed.
	EvtSteerInjected EventType = "steer:injected"
	// EvtFollowUpRaised fires when a follow-up question is raised.
	EvtFollowUpRaised EventType = "followup:raised"
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
