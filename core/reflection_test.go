package core

import (
	"strings"
	"testing"
)

func TestReflectionNoIssues(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("Here is the corrected code:\n\n```go\nfunc add(a, b int) int {\n    return a + b\n}\n```\n\nThis works correctly.")
	if !result.Passed {
		t.Error("expected no issues")
	}
}

func TestReflectionSyntaxError(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("```go\nfunc broken() int {\n    // missing return\n}\n```")
	found := false
	for _, issue := range result.Issues {
		if issue.Type == "syntax" {
			found = true
		}
	}
	if !found {
		t.Error("expected syntax issue detected")
	}
}

func TestReflectionLogicContradiction(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("The code works correctly, but sometimes produces an error.")
	found := false
	for _, issue := range result.Issues {
		if issue.Type == "logic" {
			found = true
		}
	}
	if !found {
		t.Error("expected logic contradiction detected")
	}
}

func TestReflectionIncomplete(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("Here's the plan: 1. TODO: implement database\n2. Done")
	found := false
	for _, issue := range result.Issues {
		if issue.Type == "incomplete" {
			found = true
		}
	}
	if !found {
		t.Error("expected incomplete marker detected")
	}
}

func TestReflectionIncompleteWorkInProgress(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("This feature is still work in progress")
	found := false
	for _, issue := range result.Issues {
		if issue.Type == "incomplete" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'work in progress' detected as incomplete")
	}
}

func TestReflectionHallucination(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("I confirmed the file /etc/config.yaml exists on your system.")
	found := false
	for _, issue := range result.Issues {
		if issue.Type == "hallucination" {
			found = true
		}
	}
	if !found {
		t.Error("expected hallucination detected")
	}
}

func TestReflectionCorrectionPrompt(t *testing.T) {
	engine := NewReflectionEngine(2)
	issues := []ReflectionIssue{
		{Type: "syntax", Description: "Missing return", Location: "line 3", Suggestion: "Add return"},
	}
	prompt := engine.BuildCorrectionPrompt("code", issues)
	if !strings.Contains(prompt, "Missing return") {
		t.Error("prompt should mention issue")
	}
	if !strings.Contains(prompt, "Add return") {
		t.Error("prompt should include suggestion")
	}
}

func TestReflectionMaxIssues(t *testing.T) {
	engine := NewReflectionEngine(2)
	engine.MaxIssues = 2
	output := "TODO: fix this. The file xyz.go exists. The function foo() does bar.\n" +
		"The code works but has errors.\n" +
		"```go\nfunc a() {\n}\n```\n"
	result := engine.Review(output)
	if len(result.Issues) > engine.MaxIssues {
		t.Errorf("expected ≤%d issues, got %d", engine.MaxIssues, len(result.Issues))
	}
}

func TestReflectionDepthConfig(t *testing.T) {
	e1 := NewReflectionEngine(1)
	if e1.MaxRounds != 1 {
		t.Error("expected 1 round")
	}
	e2 := NewReflectionEngine(5)
	if e2.MaxRounds != 5 {
		t.Error("expected 5 rounds")
	}
	e3 := NewReflectionEngine(0)
	if e3.MaxRounds != 2 {
		t.Error("zero should default to 2")
	}
	e4 := NewReflectionEngine(-1)
	if e4.MaxRounds != 2 {
		t.Error("negative should default to 2")
	}
}

func TestReflectionPassedReset(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("clean output with no issues")
	if !result.Passed {
		t.Error("expected passed=true for clean output")
	}
	if len(result.Issues) != 0 {
		t.Error("expected 0 issues")
	}
}

func TestReflectionStrategyRegistration(t *testing.T) {
	s := GetStrategy("reflect")
	if s == nil {
		t.Fatal("reflect strategy should be registered")
	}
	if !strings.Contains(s.Name(), "reflect") {
		t.Errorf("expected reflect in name, got %s", s.Name())
	}
}

func TestReflectionDuplicateDeclaration(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("```go\nvar x int\nvar x int\n```")
	found := false
	for _, issue := range result.Issues {
		if issue.Type == "syntax" && strings.Contains(issue.Description, "Duplicate") {
			found = true
		}
	}
	if !found {
		t.Error("expected duplicate declaration detected")
	}
}

func TestReflectionUnbalancedBraces(t *testing.T) {
	engine := NewReflectionEngine(2)
	result := engine.Review("```go\nfunc f() {\n    if true {\n        return 1\n}\n```")
	found := false
	for _, issue := range result.Issues {
		if issue.Type == "syntax" && strings.Contains(issue.Description, "Unmatched") {
			found = true
		}
	}
	if !found {
		t.Error("expected unbalanced braces detected")
	}
}

func TestReflectionFauxProvider(t *testing.T) {
	// Verify reflection engine works with faux provider output
	engine := NewReflectionEngine(2)
	result := engine.Review("I've read the file main.go and it contains a TODO for error handling.")
	foundIncomplete := false
	for _, issue := range result.Issues {
		if issue.Type == "incomplete" {
			foundIncomplete = true
		}
	}
	if !foundIncomplete {
		t.Error("expected TODO marker to be detected")
	}
}
