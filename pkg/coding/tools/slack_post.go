package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/akzj/tau/core"
)

// SlackPostTool creates a Slack webhook post tool.
//
// Parameters:
//
//	webhook_url (string, optional) — Slack incoming webhook URL (default: SLACK_WEBHOOK_URL env)
//	message     (string, required) — message text to post (supports Slack mrkdwn)
//	channel     (string, optional) — override channel
//	username    (string, optional) — override bot username
func SlackPostTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"webhook_url": {"type": "string", "description": "Slack incoming webhook URL (default: SLACK_WEBHOOK_URL env)"},
			"message": {"type": "string", "description": "Message text (supports Slack mrkdwn)"},
			"channel": {"type": "string", "description": "Override channel"},
			"username": {"type": "string", "description": "Override bot username"}
		},
		"required": ["message"]
	}`)

	return core.Tool{
		Name:        "slack_post",
		Description: "Post a message to Slack via incoming webhook. Uses SLACK_WEBHOOK_URL environment variable by default.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				WebhookURL string `json:"webhook_url"`
				Message    string `json:"message"`
				Channel    string `json:"channel"`
				Username   string `json:"username"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Message == "" {
				return core.ToolResult{}, fmt.Errorf("message required")
			}

			webhookURL := args.WebhookURL
			if webhookURL == "" {
				webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
			}
			if webhookURL == "" {
				return core.ToolResult{}, fmt.Errorf("webhook_url or SLACK_WEBHOOK_URL env required")
			}

			payload := map[string]any{
				"text": args.Message,
			}
			if args.Channel != "" {
				payload["channel"] = args.Channel
			}
			if args.Username != "" {
				payload["username"] = args.Username
			}

			body, _ := json.Marshal(payload)
			req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("post: %w", err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusOK {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Slack post failed: HTTP %d - %s", resp.StatusCode, string(respBody))}},
					Details: map[string]any{"success": false, "status": resp.StatusCode},
				}, fmt.Errorf("HTTP %d", resp.StatusCode)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: "Message posted to Slack"}},
				Details: map[string]any{"success": true, "status": resp.StatusCode},
			}, nil
		},
	}
}
