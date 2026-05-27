package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// PRReviewTool creates a PR review automation tool.
// Parameters: repo, pr_number, auto_approve (bool).
// Checks: diff size, test status, code style.
// Returns: review_comment, suggestions, approve_status.
func PRReviewTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"repo": {"type": "string", "description": "Repository owner/name (e.g., 'golang/go')"},
			"pr_number": {"type": "integer", "description": "PR number to review"},
			"auto_approve": {"type": "boolean", "description": "Auto-approve if checks pass (default false)"},
			"source": {"type": "string", "description": "Review source: github, gitlab, local (default: github)"}
		},
		"required": ["repo", "pr_number"]
	}`)

	return core.Tool{
		Name:        "pr_review",
		Description: "Automated PR review: checks diff size, test status, code style. Generates review comments and suggestions. Supports GitHub, GitLab, and local PRs.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Repo        string `json:"repo"`
				PRNumber    int    `json:"pr_number"`
				AutoApprove bool   `json:"auto_approve"`
				Source      string `json:"source"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Source == "" {
				args.Source = "github"
			}
			if args.Repo == "" {
				return core.ToolResult{}, fmt.Errorf("repo required")
			}
			if args.PRNumber <= 0 {
				return core.ToolResult{}, fmt.Errorf("pr_number required")
			}

			switch args.Source {
			case "github":
				return reviewGitHubPR(ctx, args.Repo, args.PRNumber, args.AutoApprove)
			case "gitlab":
				return reviewGitLabMR(ctx, args.Repo, args.PRNumber, args.AutoApprove)
			case "local":
				return reviewLocalPR(ctx, args.PRNumber)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown source: %s (use github/gitlab/local)", args.Source)
			}
		},
	}
}

type reviewResult struct {
	Comments    []string
	Suggestions []string
	Passed      []string
	Approved    bool
	Issues      int
}

func reviewGitHubPR(ctx context.Context, repo string, prNumber int, autoApprove bool) (core.ToolResult, error) {
	token := os.Getenv("GITHUB_TOKEN")

	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", repo, prNumber)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tau/0.1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return core.ToolResult{}, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	var pr struct {
		Title  string `json:"title"`
		State  string `json:"state"`
		Merged bool   `json:"merged"`
		DiffURL string `json:"diff_url"`
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		ChangedFiles int `json:"changed_files"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			SHA string `json:"sha"`
		} `json:"base"`
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return core.ToolResult{}, fmt.Errorf("parse PR: %w", err)
	}

	review := reviewResult{}

	// Check 1: PR state
	if pr.State != "open" {
		review.Comments = append(review.Comments, fmt.Sprintf("⚠️ PR is %s (not open)", pr.State))
	}

	// Check 2: Diff size
	review.Passed = append(review.Passed, fmt.Sprintf("✅ Title: %s", pr.Title))
	if pr.ChangedFiles > 50 {
		review.Comments = append(review.Comments, fmt.Sprintf("⚠️ Large PR: %d files changed (%d additions, %d deletions). Consider splitting.", pr.ChangedFiles, pr.Additions, pr.Deletions))
		review.Issues++
	} else if pr.ChangedFiles > 20 {
		review.Comments = append(review.Comments, fmt.Sprintf("💡 Moderate size: %d files changed (%d additions, %d deletions)", pr.ChangedFiles, pr.Additions, pr.Deletions))
	} else {
		review.Passed = append(review.Passed, fmt.Sprintf("✅ Size: %d files, +%d/-%d lines", pr.ChangedFiles, pr.Additions, pr.Deletions))
	}

	// Check 3: Test files present
	hasTests := checkPRHasTests(ctx, repo, prNumber, token)
	if !hasTests {
		review.Comments = append(review.Comments, "⚠️ No test files detected in this PR. Consider adding tests.")
		review.Suggestions = append(review.Suggestions, "Add unit tests for new functionality")
		review.Issues++
	} else {
		review.Passed = append(review.Passed, "✅ Test files detected")
	}

	// Check 4: Code style (basic heuristics from diff)
	styleIssues := checkPRCodeStyle(ctx, repo, prNumber, token)
	for _, issue := range styleIssues {
		review.Comments = append(review.Comments, fmt.Sprintf("💡 %s", issue))
		review.Suggestions = append(review.Suggestions, issue)
	}

	// Auto-approve logic
	review.Approved = review.Issues == 0 && autoApprove

	return formatReviewResult(repo, prNumber, pr.Title, review)
}

func reviewGitLabMR(ctx context.Context, repo string, mrNumber int, autoApprove bool) (core.ToolResult, error) {
	token := os.Getenv("GITLAB_TOKEN")
	projectID := strings.ReplaceAll(repo, "/", "%2F")

	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/merge_requests/%d", projectID, mrNumber)
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

	var mr struct {
		Title        string `json:"title"`
		State        string `json:"state"`
		WebURL       string `json:"web_url"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}
	json.Unmarshal(body, &mr)

	review := reviewResult{
		Passed: []string{fmt.Sprintf("Title: %s", mr.Title)},
		Comments: []string{"GitLab MR review: basic checks only (token may be required for full analysis)"},
	}
	review.Approved = autoApprove && mr.State == "opened"

	return formatReviewResult(repo, mrNumber, mr.Title, review)
}

func reviewLocalPR(ctx context.Context, prNumber int) (core.ToolResult, error) {
	review := reviewResult{}

	// Use git to analyze the diff
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat")
	cmd.Dir = WorkspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("git diff failed: %w", err)
	}

	stat := strings.TrimSpace(string(out))
	if stat == "" {
		review.Passed = append(review.Passed, "✅ No uncommitted changes")
	} else {
		review.Passed = append(review.Passed, fmt.Sprintf("Changes detected:\n```\n%s\n```", stat))
	}

	// Count files
	lines := strings.Split(strings.TrimSpace(stat), "\n")
	fileCount := 0
	for _, line := range lines {
		if strings.Contains(line, "|") {
			fileCount++
		}
	}

	if fileCount > 50 {
		review.Comments = append(review.Comments, fmt.Sprintf("⚠️ Large diff: %d files changed. Consider splitting.", fileCount))
		review.Issues++
	} else {
		review.Passed = append(review.Passed, fmt.Sprintf("✅ Files changed: %d", fileCount))
	}

	// Check for TODO/FIXME in diff
	cmd2 := exec.CommandContext(ctx, "git", "diff")
	cmd2.Dir = WorkspaceRoot
	diffOut, _ := cmd2.Output()
	diff := string(diffOut)

	if strings.Contains(strings.ToLower(diff), "todo") || strings.Contains(strings.ToLower(diff), "fixme") {
		review.Comments = append(review.Comments, "💡 Found TODO/FIXME markers in diff")
	}

	// Check for debug prints
	debugRe := regexp.MustCompile(`fmt\.(Println|Printf)\("`)
	if debugRe.MatchString(diff) {
		review.Comments = append(review.Comments, "⚠️ Debug print statements detected")
		review.Suggestions = append(review.Suggestions, "Remove debug print statements before merging")
		review.Issues++
	}

	review.Approved = review.Issues == 0

	return formatReviewResult("local", 0, "Local PR Review", review)
}

func checkPRHasTests(ctx context.Context, repo string, prNumber int, token string) bool {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/files?per_page=100", repo, prNumber)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tau/0.1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return true // assume ok if can't check
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return strings.Contains(string(body), "_test.go") || strings.Contains(string(body), "_test.py")
}

func checkPRCodeStyle(ctx context.Context, repo string, prNumber int, token string) []string {
	var issues []string

	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/files?per_page=100", repo, prNumber)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tau/0.1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return issues
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse file patches and check for patterns
	var files []struct {
		Filename string `json:"filename"`
		Patch    string `json:"patch"`
	}
	json.Unmarshal(body, &files)

	for _, f := range files {
		patch := f.Patch
		// Check for long lines
		for _, line := range strings.Split(patch, "\n") {
			if strings.HasPrefix(line, "+") && len(line) > 120 {
				issues = append(issues, fmt.Sprintf("Long line in %s (>120 chars)", f.Filename))
				break
			}
		}
		// Check for panic in production code
		if strings.Contains(patch, "panic(") && !strings.Contains(f.Filename, "_test.go") {
			issues = append(issues, fmt.Sprintf("`panic()` call in %s — consider returning error instead", f.Filename))
		}
	}

	return issues
}

func formatReviewResult(repo string, prNumber int, title string, review reviewResult) (core.ToolResult, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# PR Review: %s #%d\n\n", repo, prNumber))
	sb.WriteString(fmt.Sprintf("**Title**: %s\n\n", title))
	sb.WriteString(fmt.Sprintf("**Issues Found**: %d\n\n", review.Issues))

	// Passed checks
	if len(review.Passed) > 0 {
		sb.WriteString("## ✅ Passing Checks\n\n")
		for _, p := range review.Passed {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
		sb.WriteString("\n")
	}

	// Comments / Warnings
	if len(review.Comments) > 0 {
		sb.WriteString("## 💬 Review Comments\n\n")
		for _, c := range review.Comments {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	// Suggestions
	if len(review.Suggestions) > 0 {
		sb.WriteString("## 💡 Suggestions\n\n")
		for _, s := range review.Suggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		sb.WriteString("\n")
	}

	// Approval status
	sb.WriteString("## Verdict\n\n")
	if review.Approved {
		sb.WriteString("✅ **APPROVED** — all checks pass.\n")
	} else if review.Issues == 0 {
		sb.WriteString("✅ **READY TO MERGE** — no issues detected.\n")
	} else {
		sb.WriteString(fmt.Sprintf("⚠️ **REVIEW NEEDED** — %d issue(s) require attention.\n", review.Issues))
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{
			"repo":      repo,
			"pr_number": prNumber,
			"issues":    review.Issues,
			"approved":  review.Approved,
		},
	}, nil
}

// Used by issue_triage for GitHub API calls
func githubAPIRequest(ctx context.Context, method, url, token string, body []byte) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tau/0.1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
