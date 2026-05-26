//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	vertex_ai "github.com/akzj/tau/providers/vertex-ai"
)

func TestVertexLive(t *testing.T) {
	skipIfNoEnv(t, "VERTEX_PROJECT_ID", "VERTEX_ACCESS_TOKEN")
	prov, err := vertex_ai.NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "vertex-gemini-2.5-flash", API: core.WireGoogleVertex},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}