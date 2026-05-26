//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	openai_responses "github.com/akzj/tau/providers/openai-responses"
)

func TestOpenAIResponsesLive(t *testing.T) {
	skipIfNoEnv(t, "ANTHROPIC_AUTH_TOKEN")
	prov, err := openai_responses.NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gpt-5.4", API: core.WireOpenAIResponses},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}