package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// IssueTriageTool creates an issue triage bot tool.
// Parameters: repo, labels, auto_assign (bool).
// Labels issue by content analysis (bug/feature/docs/question).
// Optionally assigns to contributors.
func IssueTriageTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"repo": {"type": "string", "description": "Repository owner/name (e.g., 'golang/go')"},
			"action": {"type": "string", "description": "Action: analyze (classify issue), triage (classify+label), batch (triage all open)"},
			"issue_number": {"type": "integer", "description": "Issue number (for analyze/triage)"},
			"labels": {"type": "array", "items": {"type": "string"}, "description": "Labels to apply (for triage action)"},
			"auto_assign": {"type": "boolean", "description": "Auto-assign to contributors based on file paths (default false)"}
		},
		"required": ["repo"]
	}`)

	return core.Tool{
		Name:        "issue_triage",
		Description: "Issue triage bot: classifies issues by content (bug/feature/docs/question), applies labels, and optionally auto-assigns contributors.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Repo        string   `json:"repo"`
				Action      string   `json:"action"`
				IssueNumber int      `json:"issue_number"`
				Labels      []string `json:"labels"`
				AutoAssign  bool     `json:"auto_assign"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Repo == "" {
				return core.ToolResult{}, fmt.Errorf("repo required")
			}
			if args.Action == "" {
				args.Action = "analyze"
			}

			token := os.Getenv("GITHUB_TOKEN")

			switch args.Action {
			case "analyze":
				if args.IssueNumber <= 0 {
					return core.ToolResult{}, fmt.Errorf("issue_number required for analyze")
				}
				return analyzeIssue(ctx, token, args.Repo, args.IssueNumber)
			case "triage":
				if args.IssueNumber <= 0 {
					return core.ToolResult{}, fmt.Errorf("issue_number required for triage")
				}
				return triageIssue(ctx, token, args.Repo, args.IssueNumber, args.Labels, args.AutoAssign)
			case "batch":
				return batchTriage(ctx, token, args.Repo, args.Labels, args.AutoAssign)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use analyze/triage/batch)", args.Action)
			}
		},
	}
}

type issueAnalysis struct {
	Number    int
	Title     string
	Body      string
	Type      string   // bug, feature, docs, question
	Labels    []string
	Suggested []string
	Assign    string
}

func analyzeIssue(ctx context.Context, token, repo string, number int) (core.ToolResult, error) {
	analysis, err := fetchAndClassify(ctx, token, repo, number)
	if err != nil {
		return core.ToolResult{}, err
	}
	return formatIssueAnalysisResult(analysis), nil
}

func triageIssue(ctx context.Context, token, repo string, number int, labels []string, autoAssign bool) (core.ToolResult, error) {
	analysis, err := fetchAndClassify(ctx, token, repo, number)
	if err != nil {
		return core.ToolResult{}, err
	}

	if len(labels) > 0 {
		analysis.Labels = labels
	} else {
		analysis.Labels = analysis.Suggested
	}

	// Apply labels via GitHub API
	if token != "" {
		labelResults := applyLabels(ctx, token, repo, number, analysis.Labels)
		analysis.Labels = labelResults
	}

	// Auto-assign
	if autoAssign && token != "" {
		assignee := autoAssignIssue(ctx, token, repo, number, analysis)
		analysis.Assign = assignee
	}

	return formatIssueAnalysisResult(analysis), nil
}

func batchTriage(ctx context.Context, token, repo string, labels []string, autoAssign bool) (core.ToolResult, error) {
	if token == "" {
		return core.ToolResult{}, fmt.Errorf("GITHUB_TOKEN required for batch triage")
	}

	// Get open issues
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues?state=open&per_page=20&labels=none", repo)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "tau/0.1")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var issues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	json.Unmarshal(body, &issues)

	var results []string
	for _, iss := range issues {
		analysis, err := fetchAndClassify(ctx, token, repo, iss.Number)
		if err != nil {
			results = append(results, fmt.Sprintf("- #%d: ERROR — %v", iss.Number, err))
			continue
		}

		if len(labels) > 0 {
			analysis.Labels = labels
		}
		applyLabels(ctx, token, repo, iss.Number, analysis.Labels)
		results = append(results, fmt.Sprintf("- #%d: **%s** → type=%s labels=%v", iss.Number, analysis.Title, analysis.Type, analysis.Labels))
	}

	output := fmt.Sprintf("## Batch Triage: %s\n\n%d issues processed:\n\n", repo, len(results))
	output += strings.Join(results, "\n")

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{"repo": repo, "processed": len(results)},
	}, nil
}

func fetchAndClassify(ctx context.Context, token, repo string, number int) (issueAnalysis, error) {
	analysis := issueAnalysis{Number: number}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", repo, number)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tau/0.1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return analysis, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var issue struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	json.Unmarshal(body, &issue)

	analysis.Title = issue.Title
	analysis.Body = issue.Body

	// Classify
	analysis.Type, analysis.Suggested = classifyIssue(issue.Title, issue.Body)

	return analysis, nil
}

func classifyIssue(title, body string) (string, []string) {
	combined := strings.ToLower(title + " " + body)

	// Check for bug indicators
	bugWords := []string{"bug", "error", "crash", "fail", "broken", "doesn't work", "not working", "incorrect", "wrong", "panic", "exception", "stack trace", "race condition"}
	featureWords := []string{"feature", "add", "new", "implement", "support", "enhancement", "request", "would be nice", "proposal", "idea"}
	docsWords := []string{"document", "readme", "typo", "spelling", "clarify", "example", "guide", "tutorial", "wiki"}
	questionWords := []string{"how to", "how do", "what is", "why", "question", "help", "confused", "explain"}
	securityWords := []string{"security", "vulnerability", "exploit", "cve", "injection", "xss", "csrf"}

	// Score-based classification
	type score struct {
		category string
		matches  int
	}
	var scores []score

	for _, w := range bugWords {
		if strings.Contains(combined, w) {
			scores = append(scores, score{"bug", len(bugWords) - indexOf(bugWords, w)})
		}
	}
	for _, w := range featureWords {
		if strings.Contains(combined, w) {
			scores = append(scores, score{"feature", len(featureWords) - indexOf(featureWords, w)})
		}
	}
	for _, w := range docsWords {
		if strings.Contains(combined, w) {
			scores = append(scores, score{"docs", len(docsWords) - indexOf(docsWords, w)})
		}
	}
	for _, w := range questionWords {
		if strings.Contains(combined, w) {
			scores = append(scores, score{"question", len(questionWords) - indexOf(questionWords, w)})
		}
	}
	for _, w := range securityWords {
		if strings.Contains(combined, w) {
			scores = append(scores, score{"security", len(securityWords) - indexOf(securityWords, w)})
		}
	}

	// Also check conventional labels in the body
	labelRe := regexp.MustCompile(`(?i)\b(bug|feature|enhancement|documentation|question|help wanted|good first issue)\b`)
	if matches := labelRe.FindAllString(combined, -1); len(matches) > 0 {
		switch strings.ToLower(matches[0]) {
		case "bug":
			scores = append(scores, score{"bug", 100})
		case "feature", "enhancement":
			scores = append(scores, score{"feature", 100})
		case "documentation":
			scores = append(scores, score{"docs", 100})
		case "question", "help wanted":
			scores = append(scores, score{"question", 100})
		case "good first issue":
			scores = append(scores, score{"feature", 50})
		}
	}

	// Determine best match
	catCounts := map[string]int{}
	bestScore := 0
	for _, s := range scores {
		catCounts[s.category] += s.matches
		if catCounts[s.category] > bestScore {
			bestScore = catCounts[s.category]
		}
	}

	issueType := "question" // default
	if bestScore > 0 {
		for cat, count := range catCounts {
			if count == bestScore {
				issueType = cat
				break
			}
		}
	}

	// Determine suggested labels
	var labels []string
	switch issueType {
	case "bug":
		labels = []string{"bug", "triage"}
	case "feature":
		labels = []string{"enhancement", "triage"}
	case "docs":
		labels = []string{"documentation", "triage"}
	case "question":
		labels = []string{"question"}
	case "security":
		labels = []string{"security", "bug", "high-priority"}
	}

	return issueType, labels
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func applyLabels(ctx context.Context, token, repo string, number int, labels []string) []string {
	if len(labels) == 0 {
		return labels
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/labels", repo, number)
	payload, _ := json.Marshal(map[string][]string{"labels": labels})

	req, _ := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(payload)))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tau/0.1")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return labels // return suggested labels
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return labels
	}

	// Read applied labels
	body, _ := io.ReadAll(resp.Body)
	var applied []struct {
		Name string `json:"name"`
	}
	json.Unmarshal(body, &applied)

	var result []string
	for _, l := range applied {
		result = append(result, l.Name)
	}
	return result
}

func autoAssignIssue(ctx context.Context, token, repo string, number int, analysis issueAnalysis) string {
	// Try to find contributors via GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/contributors?per_page=5", repo)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "tau/0.1")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var contributors []struct {
		Login string `json:"login"`
	}
	json.Unmarshal(body, &contributors)

	if len(contributors) == 0 {
		return ""
	}

	// Assign to first contributor (simple strategy)
	assignee := contributors[0].Login

	// Apply assignee via GitHub API
	assignURL := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/assignees", repo, number)
	payload, _ := json.Marshal(map[string][]string{"assignees": {assignee}})

	assignReq, _ := http.NewRequestWithContext(ctx, "POST", assignURL, strings.NewReader(string(payload)))
	assignReq.Header.Set("Accept", "application/vnd.github+json")
	assignReq.Header.Set("Authorization", "Bearer "+token)
	assignReq.Header.Set("Content-Type", "application/json")
	assignReq.Header.Set("User-Agent", "tau/0.1")

	client2 := &http.Client{Timeout: 15 * time.Second}
	assignResp, err := client2.Do(assignReq)
	if err != nil {
		return assignee + " (suggested, API error)"
	}
	defer assignResp.Body.Close()

	if assignResp.StatusCode >= 300 {
		return assignee + " (suggested)"
	}

	return assignee + " (assigned)"
}

func formatIssueAnalysisResult(analysis issueAnalysis) core.ToolResult {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Issue Triage: #%d\n\n", analysis.Number))
	sb.WriteString(fmt.Sprintf("**Title**: %s\n\n", analysis.Title))
	sb.WriteString(fmt.Sprintf("**Classification**: `%s`\n\n", analysis.Type))

	if len(analysis.Suggested) > 0 {
		sb.WriteString("**Suggested Labels**: ")
		for i, l := range analysis.Suggested {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("`%s`", l))
		}
		sb.WriteString("\n\n")
	}

	if len(analysis.Labels) > 0 {
		sb.WriteString("**Applied Labels**: ")
		for i, l := range analysis.Labels {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("`%s`", l))
		}
		sb.WriteString("\n\n")
	}

	if analysis.Assign != "" {
		sb.WriteString(fmt.Sprintf("**Assignee**: %s\n\n", analysis.Assign))
	}

	// Show body preview
	if len(analysis.Body) > 0 {
		preview := analysis.Body
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		sb.WriteString(fmt.Sprintf("### Issue Body Preview\n```\n%s\n```\n", preview))
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{
			"issue":     analysis.Number,
			"type":      analysis.Type,
			"labels":    analysis.Labels,
			"suggested": analysis.Suggested,
			"assignee":  analysis.Assign,
		},
	}
}
