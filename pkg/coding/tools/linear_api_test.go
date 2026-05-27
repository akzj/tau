package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestLinearAPITool_ListIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"issues":{"nodes":[{"id":"i1","title":"Test issue","identifier":"T-1"}]}}}`))
	}))
	defer srv.Close()

	os.Setenv("LINEAR_API_KEY", "test-key")
	defer os.Unsetenv("LINEAR_API_KEY")

	tool := tools.LinearAPITool()
	params := map[string]any{"action": "list_issues"}
	result, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestLinearAPITool_NoAPIKey(t *testing.T) {
	os.Unsetenv("LINEAR_API_KEY")
	tool := tools.LinearAPITool()
	params := map[string]any{"action": "list_issues"}
	_, err := tool.Execute(context.Background(), "call2", params, nil)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestLinearAPITool_UnknownAction(t *testing.T) {
	os.Setenv("LINEAR_API_KEY", "test-key")
	defer os.Unsetenv("LINEAR_API_KEY")

	tool := tools.LinearAPITool()
	params := map[string]any{"action": "bogus"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}