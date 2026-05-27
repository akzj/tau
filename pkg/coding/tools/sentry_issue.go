package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// SentryIssueTool creates a Sentry issue tracking tool.
//
// Parameters:
//
//	action    (string, required) — list_issues | get_issue | resolve_issue
//	issue_id  (string, required for get_issue/resolve_issue)
//	project   (string, optional) — project slug (default: from SENTRY_PROJECT env)
//	org       (string, optional) — organization slug (default: from SENTRY_ORG env)
//	query     (string, optional) — search query for list_issues (e.g., "is:unresolved")
//	limit     (int, optional) — max issues to return (default: 25)
func SentryIssueTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list_issues, get_issue, resolve_issue"},
			"issue_id": {"type": "string", "description": "Issue ID (for get/resolve)"},
			"project": {"type": "string", "description": "Project slug (default: SENTRY_PROJECT env)"},
			"org": {"type": "string", "description": "Organization slug (default: SENTRY_ORG env)"},
			"query": {"type": "string", "description": "Search query for list_issues"},
			"limit": {"type": "integer", "description": "Max issues (default: 25)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "sentry_issue",
		Description: "Sentry issue tracker API — list, get, resolve issues. Uses SENTRY_AUTH_TOKEN, SENTRY_ORG, SENTRY_PROJECT env vars.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			authToken := os.Getenv("SENTRY_AUTH_TOKEN")
			org := os.Getenv("SENTRY_ORG")
			if authToken == "" || org == "" {
				return core.ToolResult{}, fmt.Errorf("SENTRY_AUTH_TOKEN and SENTRY_ORG env required")
			}

			var args struct {
				Action  string `json:"action"`
				IssueID string `json:"issue_id"`
				Project string `json:"project"`
				Org     string `json:"org"`
				Query   string `json:"query"`
				Limit   int    `json:"limit"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Limit == 0 {
				args.Limit = 25
			}
			if args.Org == "" {
				args.Org = org
			}
			if args.Project == "" {
				args.Project = os.Getenv("SENTRY_PROJECT")
			}

			client := &http.Client{Timeout: 15 * time.Second}
			baseURL := "https://sentry.io/api/0"

			switch args.Action {
			case "list_issues":
				return sentryListIssues(ctx, client, baseURL, authToken, args)
			case "get_issue":
				return sentryGetIssue(ctx, client, baseURL, authToken, args.IssueID)
			case "resolve_issue":
				return sentryResolveIssue(ctx, client, baseURL, authToken, args.IssueID)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list_issues/get_issue/resolve_issue)", args.Action)
			}
		},
	}
}

func sentryListIssues(ctx context.Context, client *http.Client, baseURL, token string, args struct {
	Action  string `json:"action"`
	IssueID string `json:"issue_id"`
	Project string `json:"project"`
	Org     string `json:"org"`
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
}) (core.ToolResult, error) {
	url := fmt.Sprintf("%s/projects/%s/%s/issues/?limit=%d",
		baseURL, args.Org, args.Project, args.Limit)
	if args.Query != "" {
		url += "&query=" + args.Query
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("sentry list issues: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "list_issues", "project": args.Project, "status": resp.StatusCode},
	}, nil
}

func sentryGetIssue(ctx context.Context, client *http.Client, baseURL, token, issueID string) (core.ToolResult, error) {
	if issueID == "" {
		return core.ToolResult{}, fmt.Errorf("issue_id required for get_issue")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/issues/"+issueID+"/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("sentry get issue: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(body))}},
		Details: map[string]any{"action": "get_issue", "issue_id": issueID, "status": resp.StatusCode},
	}, nil
}

func sentryResolveIssue(ctx context.Context, client *http.Client, baseURL, token, issueID string) (core.ToolResult, error) {
	if issueID == "" {
		return core.ToolResult{}, fmt.Errorf("issue_id required for resolve_issue")
	}
	body := strings.NewReader(`{"status":"resolved"}`)
	req, _ := http.NewRequestWithContext(ctx, "PUT", baseURL+"/issues/"+issueID+"/", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("sentry resolve: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, string(respBody))}},
		Details: map[string]any{"action": "resolve_issue", "issue_id": issueID, "status": resp.StatusCode},
	}, nil
}