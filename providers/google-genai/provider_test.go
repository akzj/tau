package google_genai

import (
	"os"
	"testing"

	"github.com/akzj/tau/core"
)

func TestInterfaceSatisfaction(t *testing.T) {
	os.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")
	defer os.Unsetenv("ANTHROPIC_AUTH_TOKEN")

	prov, err := NewProvider()
	if err != nil {
		t.Skip("no API key")
	}
	var _ core.Provider = prov
}

func TestErrorOnEmptyAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
	_, err := NewProvider()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}