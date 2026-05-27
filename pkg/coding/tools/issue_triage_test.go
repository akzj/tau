package tools

import (
	"context"
	"testing"
)

func TestIssueTriageTool_MissingRepo(t *testing.T) {
	tool := IssueTriageTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing repo")
	}
}

func TestIssueTriageTool_AnalyzeNoIssueNumber(t *testing.T) {
	tool := IssueTriageTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":   "test/repo",
		"action": "analyze",
	}, nil)
	if err == nil {
		t.Error("expected error for missing issue_number")
	}
}

func TestIssueTriageTool_UnknownAction(t *testing.T) {
	tool := IssueTriageTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":   "test/repo",
		"action": "invalid",
	}, nil)
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestIssueTriageTool_ClassifyBug(t *testing.T) {
	issueType, _ := classifyIssue("App crashes on startup", "When I run the app, it panics with a nil pointer exception")
	if issueType != "bug" {
		t.Errorf("expected 'bug', got '%s'", issueType)
	}
}

func TestIssueTriageTool_ClassifyFeature(t *testing.T) {
	issueType, _ := classifyIssue("Add dark mode support", "It would be nice to have a dark mode feature")
	if issueType != "feature" {
		t.Errorf("expected 'feature', got '%s'", issueType)
	}
}

func TestIssueTriageTool_ClassifyDocs(t *testing.T) {
	issueType, _ := classifyIssue("Typo in README", "There is a spelling error in the documentation")
	if issueType != "docs" {
		t.Errorf("expected 'docs', got '%s'", issueType)
	}
}

func TestIssueTriageTool_ClassifyQuestion(t *testing.T) {
	issueType, _ := classifyIssue("How do I configure the database?", "I'm confused about how to set up the connection string")
	if issueType != "question" {
		t.Errorf("expected 'question', got '%s'", issueType)
	}
}

func TestIssueTriageTool_ClassifySecurity(t *testing.T) {
	issueType, _ := classifyIssue("Security vulnerability in auth", "There is a potential injection vector in the login form")
	if issueType != "security" {
		t.Errorf("expected 'security', got '%s'", issueType)
	}
}

func TestIssueTriageTool_AnalyzeGitHubNoToken(t *testing.T) {
	tool := IssueTriageTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":         "nonexistent/repo",
		"action":       "analyze",
		"issue_number": 99999,
	}, nil)
	if err != nil {
		return
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}

func TestIssueTriageTool_DefaultAction(t *testing.T) {
	tool := IssueTriageTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo": "test/repo",
	}, nil)
	if err == nil {
		t.Error("expected error for missing issue_number with default analyze action")
	}
}