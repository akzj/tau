//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	anthropic_messages "github.com/akzj/tau/providers/anthropic-messages"
)

func TestAnthropicLive(t *testing.T) {
	skipIfNoEnv(t, "ANTHROPIC_AUTH_TOKEN")
	prov, err := anthropic_messages.NewAnthropicMessagesProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "claude-sonnet-4-6", API: core.WireAnthropicMessages},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}