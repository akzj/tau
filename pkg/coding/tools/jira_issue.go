package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// JiraIssueTool creates a Jira issue CRUD tool.
//
// Parameters:
//
//	action   (string, required) — get | create | update | search
//	issue_key (string, required for get/update) — Jira issue key (e.g., "PROJ-123")
//	summary  (string, required for create) — issue summary
//	description (string, optional) — issue description
//	project  (string, required for create) — project key
//	issuetype (string, optional) — issue type (default: Task)
//	jql      (string, required for search) — JQL query
//	url      (string, optional) — Jira base URL (default: JIRA_URL env)
//	token    (string, optional) — API token (default: JIRA_TOKEN env)
func JiraIssueTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: get, create, update, search"},
			"issue_key": {"type": "string", "description": "Issue key (e.g., PROJ-123) for get/update"},
			"summary": {"type": "string", "description": "Issue summary (for create)"},
			"description": {"type": "string", "description": "Issue description"},
			"project": {"type": "string", "description": "Project key (for create)"},
			"issuetype": {"type": "string", "description": "Issue type (default: Task)"},
			"jql": {"type": "string", "description": "JQL query string (for search)"},
			"url": {"type": "string", "description": "Jira base URL (default: JIRA_URL env)"},
			"token": {"type": "string", "description": "API token (default: JIRA_TOKEN env)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "jira_issue",
		Description: "Interact with Jira issues: get, create, update, search. Uses JIRA_URL and JIRA_TOKEN environment variables.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action      string `json:"action"`
				IssueKey    string `json:"issue_key"`
				Summary     string `json:"summary"`
				Description string `json:"description"`
				Project     string `json:"project"`
				IssueType   string `json:"issuetype"`
				JQL         string `json:"jql"`
				URL         string `json:"url"`
				Token       string `json:"token"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			baseURL := args.URL
			if baseURL == "" {
				baseURL = os.Getenv("JIRA_URL")
			}
			token := args.Token
			if token == "" {
				token = os.Getenv("JIRA_TOKEN")
			}
			if baseURL == "" || token == "" {
				return core.ToolResult{}, fmt.Errorf("JIRA_URL and JIRA_TOKEN required (via args or env)")
			}

			baseURL = strings.TrimRight(baseURL, "/")
			auth := base64.StdEncoding.EncodeToString([]byte("tau:" + token))
			client := &http.Client{Timeout: 30 * time.Second}

			switch args.Action {
			case "get":
				return jiraGet(ctx, client, baseURL, auth, args.IssueKey)
			case "create":
				return jiraCreate(ctx, client, baseURL, auth, args)
			case "update":
				return jiraUpdate(ctx, client, baseURL, auth, args)
			case "search":
				return jiraSearch(ctx, client, baseURL, auth, args.JQL)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use get/create/update/search)", args.Action)
			}
		},
	}
}

func jiraGet(ctx context.Context, client *http.Client, baseURL, auth, key string) (core.ToolResult, error) {
	if key == "" {
		return core.ToolResult{}, fmt.Errorf("issue_key required for get")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/rest/api/3/issue/%s", baseURL, key), nil)
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("jira get: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: string(body)}},
		Details: map[string]any{"action": "get", "key": key, "success": resp.StatusCode == 200, "status": resp.StatusCode},
	}, nil
}

func jiraCreate(ctx context.Context, client *http.Client, baseURL, auth string, args struct {
	Action      string `json:"action"`
	IssueKey    string `json:"issue_key"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Project     string `json:"project"`
	IssueType   string `json:"issuetype"`
	JQL         string `json:"jql"`
	URL         string `json:"url"`
	Token       string `json:"token"`
}) (core.ToolResult, error) {
	if args.Project == "" || args.Summary == "" {
		return core.ToolResult{}, fmt.Errorf("project and summary required for create")
	}
	if args.IssueType == "" {
		args.IssueType = "Task"
	}

	payload := map[string]any{
		"fields": map[string]any{
			"project":     map[string]string{"key": args.Project},
			"summary":     args.Summary,
			"description": args.Description,
			"issuetype":   map[string]string{"name": args.IssueType},
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/rest/api/3/issue", bytes.NewReader(body))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("jira create: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: string(respBody)}},
		Details: map[string]any{"action": "create", "success": resp.StatusCode == 201, "status": resp.StatusCode},
	}, nil
}

func jiraUpdate(ctx context.Context, client *http.Client, baseURL, auth string, args struct {
	Action      string `json:"action"`
	IssueKey    string `json:"issue_key"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Project     string `json:"project"`
	IssueType   string `json:"issuetype"`
	JQL         string `json:"jql"`
	URL         string `json:"url"`
	Token       string `json:"token"`
}) (core.ToolResult, error) {
	if args.IssueKey == "" {
		return core.ToolResult{}, fmt.Errorf("issue_key required for update")
	}

	payload := map[string]any{"fields": map[string]any{}}
	if args.Summary != "" {
		payload["fields"].(map[string]any)["summary"] = args.Summary
	}
	if args.Description != "" {
		payload["fields"].(map[string]any)["description"] = args.Description
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "PUT", fmt.Sprintf("%s/rest/api/3/issue/%s", baseURL, args.IssueKey), bytes.NewReader(body))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("jira update: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: string(respBody)}},
		Details: map[string]any{"action": "update", "key": args.IssueKey, "success": resp.StatusCode == 204, "status": resp.StatusCode},
	}, nil
}

func jiraSearch(ctx context.Context, client *http.Client, baseURL, auth, jql string) (core.ToolResult, error) {
	if jql == "" {
		return core.ToolResult{}, fmt.Errorf("jql required for search")
	}
	payload, _ := json.Marshal(map[string]string{"jql": jql})

	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/rest/api/3/search", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("jira search: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: string(respBody)}},
		Details: map[string]any{"action": "search", "jql": jql, "success": resp.StatusCode == 200, "status": resp.StatusCode},
	}, nil
}
