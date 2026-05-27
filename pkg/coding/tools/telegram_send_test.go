package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestTelegramSendTool_SendMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":{"message_id":123}}`))
	}))
	defer srv.Close()

	os.Setenv("TELEGRAM_BOT_TOKEN", "test-bot")
	os.Setenv("TELEGRAM_CHAT_ID", "123456")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")
	defer os.Unsetenv("TELEGRAM_CHAT_ID")

	tool := tools.TelegramSendTool()
	params := map[string]any{"action": "send_message", "text": "Hello Telegram"}
	result, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content[0].Text == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestTelegramSendTool_NoToken(t *testing.T) {
	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("TELEGRAM_CHAT_ID")
	tool := tools.TelegramSendTool()
	params := map[string]any{"action": "send_message", "text": "Hello"}
	_, err := tool.Execute(context.Background(), "call2", params, nil)
	if err == nil {
		t.Fatal("expected error for missing token/chat_id")
	}
}

func TestTelegramSendTool_UnknownAction(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-bot")
	os.Setenv("TELEGRAM_CHAT_ID", "123456")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")
	defer os.Unsetenv("TELEGRAM_CHAT_ID")

	tool := tools.TelegramSendTool()
	params := map[string]any{"action": "bogus"}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}