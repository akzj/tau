// Package testutil provides shared test harness and helpers for integration tests.
package testutil

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

// Harness provides a pre-configured integration test environment.
type Harness struct {
	Prov      *faux.Provider
	Loop      core.Loop
	Workspace string
}

// NewHarness creates a test harness with a faux provider, loop, and temp workspace.
func NewHarness(t testing.TB) *Harness {
	return &Harness{
		Prov:      faux.New(),
		Loop:      core.NewLoop(),
		Workspace: t.TempDir(),
	}
}

// NewSession creates a core.Session pre-wired to this harness's faux provider.
func (h *Harness) NewSession(ctx context.Context) (*core.Session, error) {
	return core.NewSession(ctx, core.SessionOptions{
		Provider:     h.Prov,
		DefaultModel: core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
	})
}

// NewSessionWithMemory creates a core.Session with the 3-layer memory system enabled.
func (h *Harness) NewSessionWithMemory(ctx context.Context) (*core.Session, error) {
	memDir := h.Workspace + "/memory"
	return core.NewSession(ctx, core.SessionOptions{
		Provider:     h.Prov,
		DefaultModel: core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		MemoryDir:    memDir,
	})
}

// QueueSimple queues a single successful streaming response.
func (h *Harness) QueueSimple(text string) {
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: text},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
}

// QueueToolCall queues a tool call followed by a response.
// The tool call uses toolName and sends raw JSON args.
func (h *Harness) QueueToolCall(toolName string) {
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: toolName},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"v":"test"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
}
