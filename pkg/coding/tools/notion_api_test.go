package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestNotionAPITool_ListDatabases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer token, got: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","results":[{"id":"db1","title":[{"text":{"content":"My DB"}}]}]}`))
	}))
	defer srv.Close()

	os.Setenv("NOTION_TOKEN", "test-token")
	defer os.Unsetenv("NOTION_TOKEN")

	tool := tools.NotionAPITool()
	params := map[string]any{"action": "list_databases"}
	result, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestNotionAPITool_NoToken(t *testing.T) {
	os.Unsetenv("NOTION_TOKEN")
	tool := tools.NotionAPITool()
	params := map[string]any{"action": "list_databases"}
	_, err := tool.Execute(context.Background(), "call2", params, nil)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestNotionAPITool_UnknownAction(t *testing.T) {
	os.Setenv("NOTION_TOKEN", "test-token")
	defer os.Unsetenv("NOTION_TOKEN")

	tool := tools.NotionAPITool()
	params := map[string]any{"action": "bogus"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}