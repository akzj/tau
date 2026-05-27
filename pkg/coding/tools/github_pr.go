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

// GitHubPRTool creates a GitHub Pull Requests CRUD+merge tool.
//
// Parameters:
//
//	action (string, required) — list | create | merge
//	repo   (string, required) — owner/name
//	title  (string, for create) — PR title
//	head   (string, for create) — source branch
//	base   (string, for create) — target branch
//	number (int, for merge) — PR number
//
// Requires GITHUB_TOKEN environment variable.
func GitHubPRTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: list, create, merge"},
			"repo": {"type": "string", "description": "Repository owner/name"},
			"title": {"type": "string", "description": "PR title (for create)"},
			"head": {"type": "string", "description": "Source branch (for create)"},
			"base": {"type": "string", "description": "Target branch (for create, default: main)"},
			"number": {"type": "integer", "description": "PR number (for merge)"}
		},
		"required": ["action", "repo"]
	}`)

	return core.Tool{
		Name:        "github_pr",
		Description: "GitHub Pull Requests: list, create, merge. Requires GITHUB_TOKEN env var.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				Repo   string `json:"repo"`
				Title  string `json:"title"`
				Head   string `json:"head"`
				Base   string `json:"base"`
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
				return githubListPRs(ctx, client, token, args.Repo)
			case "create":
				return githubCreatePR(ctx, client, token, args.Repo, args.Title, args.Head, args.Base)
			case "merge":
				return githubMergePR(ctx, client, token, args.Repo, args.Number)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list/create/merge)", args.Action)
			}
		},
	}
}

func githubListPRs(ctx context.Context, client *http.Client, token, repo string) (core.ToolResult, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls?state=open&per_page=10", repo)
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

	var prs []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
		URL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &prs); err != nil {
		return core.ToolResult{}, fmt.Errorf("parse response: %w", err)
	}

	output := fmt.Sprintf("## Pull Requests for %s\n\n", repo)
	if len(prs) == 0 {
		output += "No open PRs."
	} else {
		for _, pr := range prs {
			output += fmt.Sprintf("- #%d: **%s** (%s → %s) %s\n", pr.Number, pr.Title, pr.Head.Ref, "base", pr.URL)
		}
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{"repo": repo, "count": len(prs)},
	}, nil
}

func githubCreatePR(ctx context.Context, client *http.Client, token, repo, title, head, base string) (core.ToolResult, error) {
	if title == "" {
		return core.ToolResult{}, fmt.Errorf("title required for create")
	}
	if head == "" {
		return core.ToolResult{}, fmt.Errorf("head branch required for create")
	}
	if base == "" {
		base = "main"
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls", repo)
	payload := map[string]string{
		"title": title,
		"head":  head,
		"base":  base,
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

	var pr struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
	}
	json.Unmarshal(respBody, &pr)

	output := fmt.Sprintf("Created PR #%d: %s\n%s", pr.Number, title, pr.URL)
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{"repo": repo, "number": pr.Number},
	}, nil
}

func githubMergePR(ctx context.Context, client *http.Client, token, repo string, number int) (core.ToolResult, error) {
	if number == 0 {
		return core.ToolResult{}, fmt.Errorf("PR number required for merge")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/merge", repo, number)
	payload := map[string]string{"merge_method": "merge"}
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonBody))
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
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Merged PR #%d in %s", number, repo)}},
		Details: map[string]any{"repo": repo, "number": number},
	}, nil
}
