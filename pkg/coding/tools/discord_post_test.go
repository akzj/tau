package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestDiscordPostTool_SendMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	os.Setenv("DISCORD_WEBHOOK_URL", srv.URL)
	defer os.Unsetenv("DISCORD_WEBHOOK_URL")

	tool := tools.DiscordPostTool()
	params := map[string]any{"message": "Hello from test"}
	result, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Details["success"] != true {
		t.Fatal("expected success=true")
	}
}

func TestDiscordPostTool_NoURL(t *testing.T) {
	os.Unsetenv("DISCORD_WEBHOOK_URL")
	tool := tools.DiscordPostTool()
	params := map[string]any{"message": "Hello"}
	_, err := tool.Execute(context.Background(), "call2", params, nil)
	if err == nil {
		t.Fatal("expected error for missing webhook URL")
	}
}

func TestDiscordPostTool_WithEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	tool := tools.DiscordPostTool()
	params := map[string]any{
		"webhook_url": srv.URL,
		"message":     "test",
		"embed_title": "Title",
		"embed_desc":  "Description",
		"embed_color": 0xFF0000,
	}
	result, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Details["success"] != true {
		t.Fatal("expected success=true")
	}
}