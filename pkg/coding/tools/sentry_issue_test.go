package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestSentryIssueTool_NoCredentials(t *testing.T) {
	os.Unsetenv("SENTRY_AUTH_TOKEN")
	os.Unsetenv("SENTRY_ORG")

	tool := tools.SentryIssueTool()
	params := map[string]any{"action": "list_issues"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestSentryIssueTool_ListIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"1","title":"Test Error","status":"unresolved"}]`))
	}))
	defer srv.Close()

	os.Setenv("SENTRY_AUTH_TOKEN", "test-token")
	os.Setenv("SENTRY_ORG", "test-org")
	os.Setenv("SENTRY_PROJECT", "test-project")
	defer os.Unsetenv("SENTRY_AUTH_TOKEN")
	defer os.Unsetenv("SENTRY_ORG")
	defer os.Unsetenv("SENTRY_PROJECT")

	tool := tools.SentryIssueTool()
	params := map[string]any{"action": "list_issues"}
	result, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestSentryIssueTool_GetIssueNoID(t *testing.T) {
	os.Setenv("SENTRY_AUTH_TOKEN", "test-token")
	os.Setenv("SENTRY_ORG", "test-org")
	defer os.Unsetenv("SENTRY_AUTH_TOKEN")
	defer os.Unsetenv("SENTRY_ORG")

	tool := tools.SentryIssueTool()
	params := map[string]any{"action": "get_issue"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for missing issue_id")
	}
}