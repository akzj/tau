package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJiraIssueTool_MissingAuth(t *testing.T) {
	t.Setenv("JIRA_URL", "")
	t.Setenv("JIRA_TOKEN", "")

	tool := JiraIssueTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "get", "issue_key": "PROJ-1",
	}, nil)
	if err == nil {
		t.Error("expected error for missing auth")
	}
}

func TestJiraIssueTool_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"key": "PROJ-1", "fields": map[string]any{"summary": "Test"}})
	}))
	defer srv.Close()

	tool := JiraIssueTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "get", "issue_key": "PROJ-1",
		"url": srv.URL, "token": "test-token",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}
}

func TestJiraIssueTool_UnknownAction(t *testing.T) {
	tool := JiraIssueTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "invalid", "url": "http://example.com", "token": "x",
	}, nil)
	if err == nil {
		t.Error("expected error for unknown action")
	}
}
