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

// GitHubIssueTool creates a GitHub Issues CRUD tool.
//
// Parameters:
//
//	action (string, required) — list | create | update | close
//	repo   (string, required) — owner/name (e.g., "golang/go")
//	title  (string, for create/update) — issue title
//	body   (string, optional) — issue body
//	number (int, for update/close) — issue number
//
// Requires GITHUB_TOKEN environment variable.
func GitHubIssueTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list, create, update, close"},
			"repo": {"type": "string", "description": "Repository owner/name (e.g., 'golang/go')"},
			"title": {"type": "string", "description": "Issue title (for create/update)"},
			"body": {"type": "string", "description": "Issue body (optional)"},
			"number": {"type": "integer", "description": "Issue number (for update/close)"}
		},
		"required": ["action", "repo"]
	}`)

	return core.Tool{
		Name:        "github_issue",
		Description: "GitHub Issues CRUD: list, create, update, close. Requires GITHUB_TOKEN env var.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				Repo   string `json:"repo"`
				Title  string `json:"title"`
				Body   string `json:"body"`
				Number int    `json:"number"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Repo == "" {
				return core.ToolResult{}, fmt.Errorf("repo required")
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return core.ToolResult{}, fmt.Errorf("GITHUB_TOKEN not set")
			}

			client := &http.Client{Timeout: 15 * time.Second}

			switch args.Action {
			case "list":
				return githubListIssues(ctx, client, token, args.Repo)
			case "create":
				return githubCreateIssue(ctx, client, token, args.Repo, args.Title, args.Body)
			case "update":
				return githubUpdateIssue(ctx, client, token, args.Repo, args.Number, args.Title, args.Body)
			case "close":
				return githubCloseIssue(ctx, client, token, args.Repo, args.Number)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list/create/update/close)", args.Action)
			}
		},
	}
}

func githubListIssues(ctx context.Context, client *http.Client, token, repo string) (core.ToolResult, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues?state=open&per_page=10", repo)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tau/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return core.ToolResult{}, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var issues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		URL    string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &issues); err != nil {
		return core.ToolResult{}, fmt.Errorf("parse response: %w", err)
	}

	output := fmt.Sprintf("## Issues for %s\n\n", repo)
	if len(issues) == 0 {
		output += "No open issues."
	} else {
		for _, iss := range issues {
			output += fmt.Sprintf("- #%d: **%s** [%s] %s\n", iss.Number, iss.Title, iss.State, iss.URL)
		}
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{"repo": repo, "count": len(issues)},
	}, nil
}

func githubCreateIssue(ctx context.Context, client *http.Client, token, repo, title, body string) (core.ToolResult, error) {
	if title == "" {
		return core.ToolResult{}, fmt.Errorf("title required for create")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues", repo)
	payload := map[string]string{"title": title}
	if body != "" {
		payload["body"] = body
	}
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tau/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return core.ToolResult{}, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var issue struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
	}
	json.Unmarshal(respBody, &issue)

	output := fmt.Sprintf("Created issue #%d: %s\n%s", issue.Number, title, issue.URL)
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{"repo": repo, "number": issue.Number},
	}, nil
}

func githubUpdateIssue(ctx context.Context, client *http.Client, token, repo string, number int, title, body string) (core.ToolResult, error) {
	if number == 0 {
		return core.ToolResult{}, fmt.Errorf("issue number required for update")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repo, number)
	payload := map[string]string{}
	if title != "" {
		payload["title"] = title
	}
	if body != "" {
		payload["body"] = body
	}
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tau/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return core.ToolResult{}, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Updated issue #%d in %s", number, repo)}},
		Details: map[string]any{"repo": repo, "number": number},
	}, nil
}

func githubCloseIssue(ctx context.Context, client *http.Client, token, repo string, number int) (core.ToolResult, error) {
	if number == 0 {
		return core.ToolResult{}, fmt.Errorf("issue number required for close")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repo, number)
	payload := map[string]string{"state": "closed"}
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tau/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return core.ToolResult{}, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Closed issue #%d in %s", number, repo)}},
		Details: map[string]any{"repo": repo, "number": number},
	}, nil
}
