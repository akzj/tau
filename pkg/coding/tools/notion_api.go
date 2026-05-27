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

var notionBaseURL = "https://api.notion.com/v1"

// NotionAPITool creates a Notion workspace API tool.
//
// Parameters:
//
//	action       (string, required) — list_databases | query_database | create_page
//	database_id  (string, required for query_database/create_page)
//	title        (string, required for create_page)
//	content      (string, optional) — page content (markdown)
func NotionAPITool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_databases, query_database, create_page"},
			"database_id": {"type": "string", "description": "Database ID (required for query/create)"},
			"title": {"type": "string", "description": "Page title (required for create)"},
			"content": {"type": "string", "description": "Page content in markdown"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "notion_api",
		Description: "Notion workspace API — list databases, query, create pages. Uses NOTION_TOKEN env var.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			token := os.Getenv("NOTION_TOKEN")
			if token == "" {
				return core.ToolResult{}, fmt.Errorf("NOTION_TOKEN env not set")
			}

			var args struct {
				Action     string `json:"action"`
				DatabaseID string `json:"database_id"`
				Title      string `json:"title"`
				Content    string `json:"content"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			client := &http.Client{Timeout: 15 * time.Second}
			var method, url string
			var body any

			switch args.Action {
			case "list_databases":
				method, url = "POST", notionBaseURL+"/search"
				body = map[string]any{"filter": map[string]string{"value": "database", "property": "object"}}
			case "query_database":
				if args.DatabaseID == "" {
					return core.ToolResult{}, fmt.Errorf("database_id required for query_database")
				}
				method, url = "POST", notionBaseURL+"/databases/"+args.DatabaseID+"/query"
				body = map[string]any{}
			case "create_page":
				if args.DatabaseID == "" || args.Title == "" {
					return core.ToolResult{}, fmt.Errorf("database_id and title required for create_page")
				}
				method, url = "POST", notionBaseURL+"/pages"
				props := map[string]any{
					"title": map[string]any{"title": []map[string]any{
						{"text": map[string]string{"content": args.Title}},
					}},
				}
				body = map[string]any{"parent": map[string]string{"database_id": args.DatabaseID}, "properties": props}
				if args.Content != "" {
					body = map[string]any{
						"parent":     map[string]string{"database_id": args.DatabaseID},
						"properties": props,
						"children": []map[string]any{
							{"object": "block", "type": "paragraph", "paragraph": map[string]any{
								"rich_text": []map[string]any{{"type": "text", "text": map[string]string{"content": args.Content}}},
							}},
						},
					}
				}
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_databases/query_database/create_page)", args.Action)
			}

			jsonBody, _ := json.Marshal(body)
			req, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBody))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Notion-Version", "2022-06-28")
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("notion: %w", err)
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