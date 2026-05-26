//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	openai_completions "github.com/akzj/tau/providers/openai-completions"
)

func TestOpenAILive(t *testing.T) {
	skipIfNoEnv(t, "ANTHROPIC_AUTH_TOKEN")
	prov, err := openai_completions.NewOpenAICompletionsProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gpt-5.4", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}