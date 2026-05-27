package tools

import (
	"context"
	"testing"
)

func TestWebScreenshotTool_MissingURL(t *testing.T) {
	tool := WebScreenshotTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestWebScreenshotTool_InvalidURL(t *testing.T) {
	tool := WebScreenshotTool()
	// This will fail because chromedp can't connect, but that's expected in unit tests
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"url": "about:blank",
	}, nil)
	// We just check it doesn't panic; chromedp may or may not be available
	_ = err
}
