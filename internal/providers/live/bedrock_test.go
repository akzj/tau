//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/providers/bedrock"
)

func TestBedrockLive(t *testing.T) {
	skipIfNoEnv(t, "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY")
	prov, err := bedrock.NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "anthropic.claude-3-5-sonnet-20241022-v2:0", API: core.WireBedrockConverse},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}