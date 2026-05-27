package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// CIStatusTool creates a CI/CD status check tool.
// Parameters: provider (github/), repo, ref/commit.
// Returns: status, conclusion, branch, url, duration.
func CIStatusTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"provider": {"type": "string", "description": "CI provider: github, gitlab (default: github)"},
			"repo": {"type": "string", "description": "Repository owner/name (e.g., 'golang/go')"},
			"ref": {"type": "string", "description": "Branch or tag ref (default: main)"},
			"commit": {"type": "string", "description": "Specific commit SHA (optional, overrides ref)"}
		},
		"required": ["repo"]
	}`)

	return core.Tool{
		Name:        "ci_status",
		Description: "Check CI/CD status for a repository. Supports GitHub Actions. Returns status, conclusion, branch, workflow URL, and duration.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Provider string `json:"provider"`
				Repo     string `json:"repo"`
				Ref      string `json:"ref"`
				Commit   string `json:"commit"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Provider == "" {
				args.Provider = "github"
			}
			if args.Ref == "" {
				args.Ref = "main"
			}
			if args.Repo == "" {
				return core.ToolResult{}, fmt.Errorf("repo required")
			}

			switch args.Provider {
			case "github":
				return checkGitHubCI(ctx, args.Repo, args.Ref, args.Commit)
			case "gitlab":
				return checkGitLabCI(ctx, args.Repo, args.Ref, args.Commit)
			case "local":
				return checkLocalCI(ctx, args.Repo)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown provider: %s (use github/gitlab/local)", args.Provider)
			}
		},
	}
}

func checkGitHubCI(ctx context.Context, repo, ref, commit string) (core.ToolResult, error) {
	token := os.Getenv("GITHUB_TOKEN")

	var url string
	if commit != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/commits/%s/check-runs", repo, commit)
	} else {
		url = fmt.Sprintf("https://api.github.com/repos/%s/commits/%s/check-runs", repo, ref)
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tau/0.1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Fall back to CLI-based check
		return checkCIFromGitLocal(ctx, repo, ref)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		CheckRuns []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
			StartedAt  string `json:"started_at"`
			CompletedAt string `json:"completed_at"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return core.ToolResult{}, fmt.Errorf("parse GitHub response: %w", err)
	}

	return formatCIResult(repo, ref, "github", result.CheckRuns)
}

func checkGitLabCI(ctx context.Context, repo, ref, commit string) (core.ToolResult, error) {
	token := os.Getenv("GITLAB_TOKEN")
	projectID := strings.ReplaceAll(repo, "/", "%2F")

	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/pipelines?ref=%s&per_page=5", projectID, ref)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", "tau/0.1")
	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("gitlab API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var pipelines []struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
		Ref    string `json:"ref"`
		WebURL string `json:"web_url"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &pipelines); err != nil {
		return core.ToolResult{}, fmt.Errorf("parse gitlab response: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## CI Status for %s (%s)\n\n", repo, "gitlab"))
	sb.WriteString(fmt.Sprintf("**Branch**: `%s`\n\n", ref))
	if len(pipelines) == 0 {
		sb.WriteString("No pipelines found.\n")
	} else {
		for _, p := range pipelines {
			icon := statusIcon(p.Status)
			sb.WriteString(fmt.Sprintf("- %s Pipeline #%d: **%s** [%s](%s)\n",
				icon, p.ID, p.Status, ref, p.WebURL))
		}
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{"provider": "gitlab", "repo": repo, "ref": ref, "count": len(pipelines)},
	}, nil
}

func checkCIFromGitLocal(ctx context.Context, repo, ref string) (core.ToolResult, error) {
	// Use git CLI to check CI status from local repo
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%H %s")
	cmd.Dir = WorkspaceRoot
	out, _ := cmd.Output()
	commitInfo := strings.TrimSpace(string(out))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## CI Status for %s (local)\n\n", repo))
	sb.WriteString(fmt.Sprintf("**Branch**: `%s`\n", ref))
	sb.WriteString(fmt.Sprintf("**Last Commit**: %s\n\n", commitInfo))
	sb.WriteString("_Connect to GitHub API with GITHUB_TOKEN for detailed CI status._\n")

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{"provider": "github", "repo": repo, "ref": ref, "source": "local"},
	}, nil
}

func checkLocalCI(ctx context.Context, path string) (core.ToolResult, error) {
	dir := path
	if path == "" || path == "." {
		dir = WorkspaceRoot
	}

	// Run go vet + go build as basic CI check
	var sb strings.Builder
	sb.WriteString("## Local CI Check\n\n")

	cmds := []struct {
		name string
		args []string
	}{
		{"go vet", []string{"vet", "./..."}},
		{"go build", []string{"build", "./..."}},
	}

	for _, c := range cmds {
		cmd := exec.CommandContext(ctx, c.args[0], c.args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		status := "✅ PASS"
		if err != nil {
			status = "❌ FAIL"
		}
		sb.WriteString(fmt.Sprintf("**%s**: %s\n", c.name, status))
		if len(out) > 0 && err != nil {
			text := string(out)
			if len(text) > 500 {
				text = text[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", text))
		} else {
			sb.WriteString("\n")
		}
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{"provider": "local", "path": dir},
	}, nil
}

func formatCIResult(repo, ref, provider string, checks []struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
	StartedAt  string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
}) (core.ToolResult, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## CI Status for %s (%s)\n\n", repo, provider))
	sb.WriteString(fmt.Sprintf("**Branch**: `%s`\n\n", ref))

	if len(checks) == 0 {
		sb.WriteString("No check runs found for this ref.\n")
		sb.WriteString("_Tip: Set GITHUB_TOKEN env var for private repos._\n")
	} else {
		passCount, failCount, pendingCount := 0, 0, 0
		for _, c := range checks {
			icon := statusIcon(c.Conclusion)
			if c.Conclusion == "" {
				icon = statusIcon(c.Status)
			}
			duration := ""
			if c.StartedAt != "" && c.CompletedAt != "" {
				start, _ := time.Parse(time.RFC3339, c.StartedAt)
				end, _ := time.Parse(time.RFC3339, c.CompletedAt)
				duration = fmt.Sprintf(" (%s)", end.Sub(start).Round(time.Second))
			}
			sb.WriteString(fmt.Sprintf("- %s **%s**: %s%s [details](%s)\n",
				icon, c.Name, c.Conclusion, duration, c.HTMLURL))

			switch c.Conclusion {
			case "success", "neutral":
				passCount++
			case "failure", "cancelled", "timed_out":
				failCount++
			default:
				pendingCount++
			}
		}

		sb.WriteString(fmt.Sprintf("\n**Summary**: %d passed, %d failed, %d pending\n", passCount, failCount, pendingCount))
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{"provider": provider, "repo": repo, "ref": ref, "checks": len(checks)},
	}, nil
}

func statusIcon(status string) string {
	switch strings.ToLower(status) {
	case "success", "passed":
		return "✅"
	case "failure", "failed":
		return "❌"
	case "pending", "running", "in_progress", "created":
		return "🔄"
	case "cancelled", "canceled":
		return "⏹️"
	case "skipped":
		return "⏭️"
	case "queued":
		return "⏳"
	default:
		return "❓"
	}
}
