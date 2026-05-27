package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestSemanticSearchTool_FindsRelevantFile(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "auth.go"), `package test

// AuthenticateUser checks credentials and returns a session token.
func AuthenticateUser(username, password string) (string, error) {
	if username == "" {
		return "", fmt.Errorf("empty username")
	}
	return "token-12345", nil
}
`)

	tool := tools.SemanticSearchTool()
	params := map[string]any{
		"query": "authentication token",
		"scope": "code",
	}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "auth.go") {
		t.Errorf("expected 'auth.go' in results, got:\n%s", text)
	}
}

func TestSemanticSearchTool_EmptyResults(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "main.go"), `package test

func CalculateSum(a, b int) int { return a + b }
`)

	tool := tools.SemanticSearchTool()
	params := map[string]any{
		"query": "zzz_nonexistent_query_term_xyz",
		"scope": "code",
	}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "No results found") {
		t.Errorf("expected 'No results found', got: %s", res.Content[0].Text)
	}
}

func TestSemanticSearchTool_RespectsTopK(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	// Create multiple matching files
	for _, fname := range []string{"a.go", "b.go", "c.go", "d.go", "e.go"} {
		writeFile(t, filepath.Join(dir, fname), "package test\nfunc MatchFunction() int { return 1 }\n")
	}

	tool := tools.SemanticSearchTool()
	params := map[string]any{
		"query":  "MatchFunction",
		"top_k":  2,
		"scope":  "code",
	}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	// "2 results found" should appear in summary
	if !strings.Contains(text, "2 results found") {
		t.Errorf("expected 2 results with top_k=2, got:\n%s", text)
	}
}

func TestSemanticSearchTool_MissingQuery(t *testing.T) {
	tool := tools.SemanticSearchTool()
	params := map[string]any{
		"scope": "code",
	}
	_, err := tool.Execute(context.Background(), "call4", params, nil)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
	if !strings.Contains(err.Error(), "query required") {
		t.Errorf("expected 'query required' error, got: %v", err)
	}
}

func TestSemanticSearchTool_DefaultScopeIsCode(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "main.go"), `package test

func HelloWorld() string { return "hello" }
`)

	tool := tools.SemanticSearchTool()
	params := map[string]any{
		"query": "HelloWorld",
	}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "main.go") {
		t.Errorf("expected 'main.go' in results with default scope, got:\n%s", text)
	}
}

func TestSemanticSearchTool_MinScore(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "main.go"), `package test

func HelloWorld() string { return "hello" }
`)

	tool := tools.SemanticSearchTool()
	// With very high min_score, nothing should match
	params := map[string]any{
		"query":     "HelloWorld",
		"min_score": 0.99,
	}
	res, err := tool.Execute(context.Background(), "call6", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "No results found") {
		t.Errorf("expected 'No results found' with high min_score, got:\n%s", text)
	}
}
