package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/akzj/tau/core"
	"github.com/chromedp/chromedp"
)

// WebScreenshotTool creates a chromedp-based screenshot tool.
//
// Parameters:
//
//	url      (string, required) — URL to screenshot
//	width    (number, optional) — viewport width (default: 1280)
//	height   (number, optional) — viewport height (default: 720)
//	full_page (bool, optional) — capture full page scroll
func WebScreenshotTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to screenshot"},
			"width": {"type": "number", "description": "Viewport width (default: 1280)"},
			"height": {"type": "number", "description": "Viewport height (default: 720)"},
			"full_page": {"type": "boolean", "description": "Capture full page scroll"}
		},
		"required": ["url"]
	}`)

	return core.Tool{
		Name:        "web_screenshot",
		Description: "Capture a screenshot of a web page using headless Chrome. Returns base64-encoded PNG.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				URL      string  `json:"url"`
				Width    int64   `json:"width"`
				Height   int64   `json:"height"`
				FullPage bool    `json:"full_page"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.URL == "" {
				return core.ToolResult{}, fmt.Errorf("url required")
			}
			if args.Width <= 0 {
				args.Width = 1280
			}
			if args.Height <= 0 {
				args.Height = 720
			}

			// Create chromedp context with timeout
			allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
				chromedp.Flag("headless", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-dev-shm-usage", true),
			)
			defer allocCancel()

			taskCtx, taskCancel := chromedp.NewContext(allocCtx)
			defer taskCancel()

			timeoutCtx, timeoutCancel := context.WithTimeout(taskCtx, 30*time.Second)
			defer timeoutCancel()

			var buf []byte
			captureAction := chromedp.FullScreenshot(&buf, 90)
			if !args.FullPage {
				captureAction = chromedp.CaptureScreenshot(&buf)
			}

			err := chromedp.Run(timeoutCtx,
				chromedp.EmulateViewport(args.Width, args.Height),
				chromedp.Navigate(args.URL),
				chromedp.WaitReady("body"),
				captureAction,
			)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("screenshot: %w", err)
			}

			b64 := base64.StdEncoding.EncodeToString(buf)
			return core.ToolResult{
				Content: []core.Content{
					{Type: "image", Data: buf, Text: fmt.Sprintf("Screenshot of %s (%dx%d, %d bytes)", args.URL, args.Width, args.Height, len(buf))},
				},
				Details: map[string]any{
					"url":          args.URL,
					"width":        args.Width,
					"height":       args.Height,
					"full_page":    args.FullPage,
					"size_bytes":   len(buf),
					"base64":       b64,
					"success":      true,
				},
			}, nil
		},
	}
}
