package tools

import (
	"context"
	"testing"
)

func TestEmailSendTool_MissingRequired(t *testing.T) {
	tool := EmailSendTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"subject": "Test",
	}, nil)
	if err == nil {
		t.Error("expected error for missing fields")
	}
}

func TestEmailSendTool_NoSMTPConfig(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("SMTP_PASSWORD", "")

	tool := EmailSendTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"to": "alice@example.com", "subject": "Test", "body": "Hello",
	}, nil)
	if err == nil {
		t.Error("expected error for missing SMTP config")
	}
}
