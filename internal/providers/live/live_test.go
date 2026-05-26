//go:build integration

package live

import (
	"os"
	"testing"

	"github.com/akzj/tau/core"
)

func skipIfNoEnv(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if os.Getenv(name) == "" {
			t.Skipf("Skipping: %s not set", name)
		}
	}
}

func assertContentEvent(t *testing.T, events <-chan core.ProviderEvent) {
	t.Helper()
	hasContent := false
	for ev := range events {
		if ev.Type == core.ProvContentDelta {
			hasContent = true
		}
		if ev.Type == core.ProvError {
			t.Errorf("provider error: %v", ev.Err)
		}
	}
	if !hasContent {
		t.Error("expected at least one content delta event")
	}
}