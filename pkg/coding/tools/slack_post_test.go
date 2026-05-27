package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackPostTool_MissingMessage(t *testing.T) {
	tool := SlackPostTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"webhook_url": "http://example.com",
	}, nil)
	if err == nil {
		t.Error("expected error for missing message")
	}
}

func TestSlackPostTool_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	tool := SlackPostTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"webhook_url": srv.URL,
		"message":     "Hello from test",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}
}

func TestSlackPostTool_MissingWebhook(t *testing.T) {
	// Clear SLACK_WEBHOOK_URL
	t.Setenv("SLACK_WEBHOOK_URL", "")

	tool := SlackPostTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"message": "test",
	}, nil)
	if err == nil {
		t.Error("expected error for missing webhook URL")
	}
}
