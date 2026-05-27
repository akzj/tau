package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/akzj/tau/core"
)

// TelegramSendTool creates a Telegram Bot API tool.
//
// Parameters:
//
//	action  (string, required) — send_message | send_photo
//	text    (string, required for send_message) — message text
//	photo   (string, required for send_photo) — photo URL or file_id
//	caption (string, optional) — photo caption
func TelegramSendTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: send_message, send_photo"},
			"text": {"type": "string", "description": "Message text (for send_message)"},
			"photo": {"type": "string", "description": "Photo URL or file_id (for send_photo)"},
			"caption": {"type": "string", "description": "Photo caption"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "telegram_send",
		Description: "Send messages via Telegram Bot API. Uses TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID env vars.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
			chatID := os.Getenv("TELEGRAM_CHAT_ID")
			if botToken == "" || chatID == "" {
				return core.ToolResult{}, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID env required")
			}

			var args struct {
				Action  string `json:"action"`
				Text    string `json:"text"`
				Photo   string `json:"photo"`
				Caption string `json:"caption"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			client := &http.Client{Timeout: 15 * time.Second}
			var apiURL string

			switch args.Action {
			case "send_message":
				if args.Text == "" {
					return core.ToolResult{}, fmt.Errorf("text required for send_message")
				}
				apiURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
				apiURL += "?chat_id=" + url.QueryEscape(chatID)
				apiURL += "&text=" + url.QueryEscape(args.Text)
				apiURL += "&parse_mode=Markdown"
			case "send_photo":
				if args.Photo == "" {
					return core.ToolResult{}, fmt.Errorf("photo required for send_photo")
				}
				apiURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", botToken)
				apiURL += "?chat_id=" + url.QueryEscape(chatID)
				apiURL += "&photo=" + url.QueryEscape(args.Photo)
				if args.Caption != "" {
					apiURL += "&caption=" + url.QueryEscape(args.Caption)
				}
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use send_message/send_photo)", args.Action)
			}

			req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("telegram: %w", err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))}},
				Details: map[string]any{"action": args.Action, "status": resp.StatusCode},
			}, nil
		},
	}
}