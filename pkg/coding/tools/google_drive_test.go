package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGoogleDriveTool_NoCredentials(t *testing.T) {
	os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
	os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	tool := tools.GoogleDriveTool()
	params := map[string]any{"action": "list_files"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestGoogleDriveTool_WithAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"files":[{"id":"f1","name":"test.txt","mimeType":"text/plain"}]}`))
	}))
	defer srv.Close()

	os.Setenv("GOOGLE_ACCESS_TOKEN", "test-token")
	defer os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	tool := tools.GoogleDriveTool()
	params := map[string]any{"action": "list_files"}
	result, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestGoogleDriveTool_UnknownAction(t *testing.T) {
	os.Setenv("GOOGLE_ACCESS_TOKEN", "test-token")
	defer os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	tool := tools.GoogleDriveTool()
	params := map[string]any{"action": "bogus"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}