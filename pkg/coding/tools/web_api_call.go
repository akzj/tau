package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// WebAPICallTool creates a REST API call tool.
//
// Parameters:
//
//	url     (string, required) — API endpoint URL
//	method  (string, optional) — HTTP method: GET, POST, PUT, DELETE (default: GET)
//	headers (object, optional) — request headers as key-value pairs
//	body    (string, optional) — request body
//	auth    (string, optional) — auth header value (e.g., "Bearer <token>" or "Basic <base64>")
//	timeout (number, optional) — timeout in seconds (default: 30)
func WebAPICallTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "API endpoint URL"},
			"method": {"type": "string", "description": "HTTP method: GET, POST, PUT, DELETE (default: GET)"},
			"headers": {"type": "object", "description": "Request headers as key-value pairs"},
			"body": {"type": "string", "description": "Request body"},
			"auth": {"type": "string", "description": "Authorization header value (e.g., Bearer <token>)"},
			"timeout": {"type": "number", "description": "Timeout in seconds (default: 30)"}
		},
		"required": ["url"]
	}`)

	return core.Tool{
		Name:        "web_api_call",
		Description: "Make REST API calls (GET/POST/PUT/DELETE) with headers, body, and auth support.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				URL     string            `json:"url"`
				Method  string            `json:"method"`
				Headers map[string]string `json:"headers"`
				Body    string            `json:"body"`
				Auth    string            `json:"auth"`
				Timeout float64           `json:"timeout"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.URL == "" {
				return core.ToolResult{}, fmt.Errorf("url required")
			}
			if args.Method == "" {
				args.Method = "GET"
			}
			method := strings.ToUpper(args.Method)
			if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" {
				return core.ToolResult{}, fmt.Errorf("unsupported method: %s (use GET/POST/PUT/DELETE/PATCH)", args.Method)
			}

			timeout := 30 * time.Second
			if args.Timeout > 0 {
				timeout = time.Duration(args.Timeout * float64(time.Second))
			}

			var bodyReader io.Reader
			if args.Body != "" {
				bodyReader = bytes.NewBufferString(args.Body)
			}

			req, err := http.NewRequestWithContext(ctx, method, args.URL, bodyReader)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("create request: %w", err)
			}

			if args.Headers != nil {
				for k, v := range args.Headers {
					req.Header.Set(k, v)
				}
			}
			if args.Auth != "" {
				req.Header.Set("Authorization", args.Auth)
			}
			if args.Body != "" && req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}

			client := &http.Client{Timeout: timeout}
			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("api call: %w", err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: formatAPIResponse(resp.StatusCode, resp.Header, respBody)}},
				Details: map[string]any{
					"url":        args.URL,
					"method":     method,
					"status":     resp.StatusCode,
					"body":       string(respBody),
					"success":    resp.StatusCode >= 200 && resp.StatusCode < 300,
					"auth_method": authMethod(args.Auth),
				},
			}, nil
		},
	}
}

func formatAPIResponse(status int, headers http.Header, body []byte) string {
	var respHeaders []string
	for k, vs := range headers {
		for _, v := range vs {
			respHeaders = append(respHeaders, fmt.Sprintf("%s: %s", k, v))
		}
	}
	return fmt.Sprintf("HTTP %d\nHeaders:\n  %s\n\nBody:\n%s", status, strings.Join(respHeaders, "\n  "), string(body))
}

func authMethod(auth string) string {
	if auth == "" {
		return "none"
	}
	if strings.HasPrefix(auth, "Bearer ") {
		return "bearer"
	}
	if strings.HasPrefix(auth, "Basic ") {
		_, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if err == nil {
			return "basic"
		}
	}
	return "custom"
}
