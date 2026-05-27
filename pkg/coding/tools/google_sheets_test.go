package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGoogleSheetsTool_NoCredentials(t *testing.T) {
	os.Unsetenv("GOOGLE_ACCESS_TOKEN")
	os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	tool := tools.GoogleSheetsTool()
	params := map[string]any{"action": "read_range", "spreadsheet_id": "abc123"}
	_, err := tool.Execute(context.Background(), "call1", params, nil)
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestGoogleSheetsTool_ReadRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"range":"Sheet1!A1:D5","values":[["Name","Age"],["Alice","30"]]}`))
	}))
	defer srv.Close()

	os.Setenv("GOOGLE_ACCESS_TOKEN", "test-token")
	defer os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	tool := tools.GoogleSheetsTool()
	params := map[string]any{"action": "read_range", "spreadsheet_id": "abc123", "range": "Sheet1!A1:D5"}
	result, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestGoogleSheetsTool_UnknownAction(t *testing.T) {
	os.Setenv("GOOGLE_ACCESS_TOKEN", "test-token")
	defer os.Unsetenv("GOOGLE_ACCESS_TOKEN")

	tool := tools.GoogleSheetsTool()
	params := map[string]any{"action": "bogus", "spreadsheet_id": "abc123"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}