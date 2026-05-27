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

// DiscordPostTool creates a Discord webhook post tool.
//
// Parameters:
//
//	webhook_url (string, optional) — Discord webhook URL (default: DISCORD_WEBHOOK_URL env)
//	message     (string, required) — message text
//	embed_title (string, optional) — embed title
//	embed_desc  (string, optional) — embed description
//	embed_color (int, optional) — embed color as decimal (default: 0)
func DiscordPostTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"webhook_url": {"type": "string", "description": "Discord webhook URL (default: DISCORD_WEBHOOK_URL env)"},
			"message": {"type": "string", "description": "Message content"},
			"embed_title": {"type": "string", "description": "Embed title"},
			"embed_desc": {"type": "string", "description": "Embed description"},
			"embed_color": {"type": "integer", "description": "Embed color (decimal)"}
		},
		"required": ["message"]
	}`)

	return core.Tool{
		Name:        "discord_post",
		Description: "Post a message to Discord via webhook. Supports embeds. Uses DISCORD_WEBHOOK_URL env var.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				WebhookURL string `json:"webhook_url"`
				Message    string `json:"message"`
				EmbedTitle string `json:"embed_title"`
				EmbedDesc  string `json:"embed_desc"`
				EmbedColor int    `json:"embed_color"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Message == "" {
				return core.ToolResult{}, fmt.Errorf("message required")
			}

			webhookURL := args.WebhookURL
			if webhookURL == "" {
				webhookURL = os.Getenv("DISCORD_WEBHOOK_URL")
			}
			if webhookURL == "" {
				return core.ToolResult{}, fmt.Errorf("webhook_url or DISCORD_WEBHOOK_URL env required")
			}

			payload := map[string]any{"content": args.Message}
			if args.EmbedTitle != "" || args.EmbedDesc != "" {
				embed := map[string]any{}
				if args.EmbedTitle != "" {
					embed["title"] = args.EmbedTitle
				}
				if args.EmbedDesc != "" {
					embed["description"] = args.EmbedDesc
				}
				if args.EmbedColor != 0 {
					embed["color"] = args.EmbedColor
				}
				payload["embeds"] = []map[string]any{embed}
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
			if resp.StatusCode >= 400 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Discord post failed: HTTP %d - %s", resp.StatusCode, string(respBody))}},
					Details: map[string]any{"success": false, "status": resp.StatusCode},
				}, fmt.Errorf("HTTP %d", resp.StatusCode)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: "Message posted to Discord"}},
				Details: map[string]any{"success": true, "status": resp.StatusCode},
			}, nil
		},
	}
}