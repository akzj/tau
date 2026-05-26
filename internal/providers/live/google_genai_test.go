//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	google_genai "github.com/akzj/tau/providers/google-genai"
)

func TestGoogleGenAILive(t *testing.T) {
	skipIfNoEnv(t, "ANTHROPIC_AUTH_TOKEN")
	prov, err := google_genai.NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gemini-3.5-flash", API: core.WireGoogleGenerativeAI},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}