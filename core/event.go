package core

import (
	"encoding/json"
	"time"
)

// --- ProviderEvent (raw from provider, internal) ---

// ProviderEventType categorises raw provider events.
type ProviderEventType string

const (
	ProvMessageStart  ProviderEventType = "message_start"
	ProvContentDelta  ProviderEventType = "content_delta"
	ProvMessageEnd    ProviderEventType = "message_end"
	ProvToolCallStart ProviderEventType = "tool_call_start"
	ProvToolCallDelta ProviderEventType = "tool_call_delta"
	ProvToolCallEnd   ProviderEventType = "tool_call_end"
	ProvThinkingDelta ProviderEventType = "thinking_delta"
	ProvThinkingEnd   ProviderEventType = "thinking_end"
	ProvError         ProviderEventType = "error"
	ProvUsage         ProviderEventType = "usage"
	ProvRawChunk      ProviderEventType = "raw_chunk"
)

// ProviderEvent is the raw event produced by a Provider.
type ProviderEvent struct {
	Type          ProviderEventType
	MessageID     string
	ContentDelta  string
	ToolCallID    string
	ToolName      string
	ToolArgsDelta string // accumulated JSON fragment
	Err           error
	Usage         *Usage `json:"usage,omitempty"` // ADD: token count info
	Raw           string `json:"raw,omitempty"`   // ADD: raw SSE chunk for debugging
}

// --- AgentEvent (high-level sealed interface, consumed by product layer) ---

// AgentEvent is a sealed interface. Only types defined in this package may implement it.
type AgentEvent interface {
	agentEventMarker()
	Timestamp() time.Time
}

// --- Role ---

// Role identifies the speaker of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// --- Concrete AgentEvent types ---

// MessageStart signals the beginning of a new assistant message.
type MessageStart struct {
	Timestamp_ time.Time
	MessageID  string
	Role       Role
}

func (MessageStart) agentEventMarker()          {}
func (e MessageStart) Timestamp() time.Time { return e.Timestamp_ }

// MessageDelta carries a chunk of content for an in-progress message.
type MessageDelta struct {
	Timestamp_   time.Time
	MessageID    string
	ContentDelta string
}

func (MessageDelta) agentEventMarker()          {}
func (e MessageDelta) Timestamp() time.Time { return e.Timestamp_ }

// MessageEnd signals completion of an assistant message.
type MessageEnd struct {
	Timestamp_ time.Time
	MessageID  string
}

func (MessageEnd) agentEventMarker()          {}
func (e MessageEnd) Timestamp() time.Time { return e.Timestamp_ }

// ToolCallStart signals the beginning of a tool call.
type ToolCallStart struct {
	Timestamp_ time.Time
	CallID     string
	ToolName   string
	Args       json.RawMessage
}

func (ToolCallStart) agentEventMarker()          {}
func (e ToolCallStart) Timestamp() time.Time { return e.Timestamp_ }

// ToolCallUpdate carries a partial tool execution result update.
type ToolCallUpdate struct {
	Timestamp_ time.Time
	CallID     string
	Partial    PartialResult
}

func (ToolCallUpdate) agentEventMarker()          {}
func (e ToolCallUpdate) Timestamp() time.Time { return e.Timestamp_ }

// ToolCallEnd signals completion of a tool call with its result.
type ToolCallEnd struct {
	Timestamp_ time.Time
	CallID     string
	Result     ToolResult
}

func (ToolCallEnd) agentEventMarker()          {}
func (e ToolCallEnd) Timestamp() time.Time { return e.Timestamp_ }

// ThinkingDelta represents a chunk of extended thinking content from the model.
// Only emitted when the provider supports thinking (Anthropic extended thinking).
type ThinkingDelta struct {
	Timestamp_ time.Time
	Content    string
}

func (ThinkingDelta) agentEventMarker()          {}
func (e ThinkingDelta) Timestamp() time.Time { return e.Timestamp_ }

// ThinkingEnd signals the end of a thinking block.
type ThinkingEnd struct {
	Timestamp_ time.Time
}

func (ThinkingEnd) agentEventMarker()          {}
func (e ThinkingEnd) Timestamp() time.Time { return e.Timestamp_ }

// TurnStart marks the beginning of a new turn in the conversation loop.
type TurnStart struct {
	Timestamp_ time.Time
	TurnID     string
}

func (TurnStart) agentEventMarker()          {}
func (e TurnStart) Timestamp() time.Time { return e.Timestamp_ }

// TurnEnd marks the completion of a turn with a reason.
type TurnEnd struct {
	Timestamp_ time.Time
	TurnID     string
	Reason     string // "complete" | "tool_calls" | "error" | "cancelled"
}

func (TurnEnd) agentEventMarker()          {}
func (e TurnEnd) Timestamp() time.Time { return e.Timestamp_ }

// ErrorEvent signals an error during agent processing.
type ErrorEvent struct {
	Timestamp_ time.Time
	Err        error
	Code       ErrorCode
}

func (ErrorEvent) agentEventMarker()          {}
func (e ErrorEvent) Timestamp() time.Time { return e.Timestamp_ }

// SteerEvent is injected by the user during a running turn to change direction.
type SteerEvent struct {
	Timestamp_ time.Time
	Message    string // user direction instruction
	SteerID    string
}

func (SteerEvent) agentEventMarker()          {}
func (e SteerEvent) Timestamp() time.Time { return e.Timestamp_ }

// FollowUpEvent is injected by the agent to ask a clarifying question mid-turn.
type FollowUpEvent struct {
	Timestamp_ time.Time
	Question   string
	FollowUpID string
}

func (FollowUpEvent) agentEventMarker()          {}
func (e FollowUpEvent) Timestamp() time.Time { return e.Timestamp_ }
