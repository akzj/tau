//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/providers/mistral"
)

func TestMistralLive(t *testing.T) {
	skipIfNoEnv(t, "MISTRAL_API_KEY")
	prov, err := mistral.NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "mistral-large", API: core.WireMistralConversations},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}