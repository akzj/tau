package extensions

import (
	"github.com/akzj/tau/core"
	"github.com/dop251/goja"
)

// Bridge connects core AgentEvents to extension event handlers.
// It translates AgentEvent types into string-named events that
// extensions registered via tau.on() can handle.
type Bridge struct {
	api *ExtensionAPI
	ctx *ExtensionContext
}

// NewBridge creates an event bridge.
func NewBridge(api *ExtensionAPI, ctx *ExtensionContext) *Bridge {
	return &Bridge{api: api, ctx: ctx}
}

// HandleAgentEvent converts a core.AgentEvent to an extension event and fires it.
func (b *Bridge) HandleAgentEvent(ev core.AgentEvent) {
	if b.api.vm == nil {
		return
	}
	vm := b.api.vm

	switch e := ev.(type) {
	// ── Turn lifecycle ──
	case core.TurnStart:
		b.fire("turn:start", vm.ToValue(map[string]interface{}{
			"turnID": e.TurnID,
		}))

	case core.TurnEnd:
		b.fire("turn:end", vm.ToValue(map[string]interface{}{
			"turnID": e.TurnID,
			"reason": e.Reason,
		}))

	// ── Message lifecycle ──
	case core.MessageStart:
		b.fire("message:start", vm.ToValue(map[string]interface{}{
			"messageID": e.MessageID,
			"role":      string(e.Role),
		}))

	case core.MessageDelta:
		b.fire("message:update", vm.ToValue(map[string]interface{}{
			"messageID": e.MessageID,
			"delta":     e.ContentDelta,
		}))

	case core.MessageEnd:
		b.fire("message:end", vm.ToValue(map[string]interface{}{
			"messageID": e.MessageID,
		}))

	// ── Tool lifecycle ──
	case core.ToolCallStart:
		b.fire("tool:start", vm.ToValue(map[string]interface{}{
			"callID":   e.CallID,
			"toolName": e.ToolName,
			"args":     string(e.Args),
		}))

	case core.ToolCallUpdate:
		b.fire("tool:update", vm.ToValue(map[string]interface{}{
			"callID": e.CallID,
		}))

	case core.ToolCallEnd:
		b.fire("tool:end", vm.ToValue(map[string]interface{}{
			"callID": e.CallID,
		}))

	// ── Errors ──
	case core.ErrorEvent:
		errMsg := ""
		if e.Err != nil {
			errMsg = e.Err.Error()
		}
		b.fire("error", vm.ToValue(map[string]interface{}{
			"error": errMsg,
		}))
	}
}

// fire is a helper that creates a goja Value from data and dispatches.
func (b *Bridge) fire(name string, val goja.Value) {
	b.api.Fire(name, val, b.ctx)
}

// API returns the bridge's ExtensionAPI.
func (b *Bridge) API() *ExtensionAPI { return b.api }

// Context returns the bridge's ExtensionContext.
func (b *Bridge) Context() *ExtensionContext { return b.ctx }
