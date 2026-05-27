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

var linearBaseURL = "https://api.linear.app/graphql"

// LinearAPITool creates a Linear issue tracker API tool.
//
// Parameters:
//
//	action     (string, required) — list_issues | create_issue | update_issue
//	team_id    (string, required for create_issue)
//	title      (string, required for create_issue)
//	description (string, optional)
//	issue_id   (string, required for update_issue)
//	status     (string, optional, for update_issue)
func LinearAPITool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_issues, create_issue, update_issue"},
			"team_id": {"type": "string", "description": "Team ID (required for create)"},
			"title": {"type": "string", "description": "Issue title (required for create)"},
			"description": {"type": "string", "description": "Issue description"},
			"issue_id": {"type": "string", "description": "Issue ID (required for update)"},
			"status": {"type": "string", "description": "Issue status (for update)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "linear_api",
		Description: "Linear issue tracker API — list, create, update issues. Uses LINEAR_API_KEY env var.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			apiKey := os.Getenv("LINEAR_API_KEY")
			if apiKey == "" {
				return core.ToolResult{}, fmt.Errorf("LINEAR_API_KEY env not set")
			}

			var args struct {
				Action      string `json:"action"`
				TeamID      string `json:"team_id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				IssueID     string `json:"issue_id"`
				Status      string `json:"status"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			var query string
			switch args.Action {
			case "list_issues":
				query = `{"query": "{ issues(first: 50) { nodes { id title identifier state { name } } } }"}`
			case "create_issue":
				if args.TeamID == "" || args.Title == "" {
					return core.ToolResult{}, fmt.Errorf("team_id and title required for create_issue")
				}
				query = fmt.Sprintf(`{"query": "mutation { issueCreate(input: { teamId: \"%s\", title: \"%s\", description: \"%s\" }) { success issue { id title identifier } } }"}`,
					args.TeamID, args.Title, args.Description)
			case "update_issue":
				if args.IssueID == "" {
					return core.ToolResult{}, fmt.Errorf("issue_id required for update_issue")
				}
				st := args.Status
				if st == "" {
					st = "In Progress"
				}
				query = fmt.Sprintf(`{"query": "mutation { issueUpdate(id: \"%s\", input: { stateId: \"%s\" }) { success issue { id title } } }"}`, args.IssueID, st)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_issues/create_issue/update_issue)", args.Action)
			}

			client := &http.Client{Timeout: 15 * time.Second}
			req, _ := http.NewRequestWithContext(ctx, "POST", linearBaseURL, bytes.NewReader([]byte(query)))
			req.Header.Set("Authorization", apiKey)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("linear: %w", err)
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