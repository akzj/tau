//go:build integration

package live

import (
	"context"
	"testing"

	"github.com/akzj/tau/core"
	azure_openai "github.com/akzj/tau/providers/azure-openai"
)

func TestAzureLive(t *testing.T) {
	skipIfNoEnv(t, "AZURE_OPENAI_ENDPOINT")
	prov, err := azure_openai.NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "azure-gpt-4o", API: core.WireAzureOpenAIResponses},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Say hello"}},
		Options: core.ProviderOptions{MaxTokens: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContentEvent(t, events)
}