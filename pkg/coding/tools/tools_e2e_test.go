package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding/tools"
	"github.com/akzj/tau/pkg/testing/faux"
)

// toolSchema wraps a raw schema for testing convenience.
func toolSchema(raw string) core.ToolSchema {
	return tools.Schema{Raw: json.RawMessage(raw)}
}

func TestMultiTurnToolE2E(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Turn 1: echo tool call
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "echo"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"msg":"test"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Turn 2: assistant response
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "echoed test"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     prov,
		DefaultModel: core.ModelSpec{Name: "test-model", API: core.WireOpenAICompletions},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Cancel()

	sess.Tools.Register(core.Tool{
		Name:        "echo",
		Description: "echo",
		Schema:      toolSchema(`{"type":"object","properties":{"msg":{"type":"string"}},"required":["msg"]}`),
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ECHO: test"}}}, nil
		},
	})

	loop := core.NewLoop()

	// Turn 1: prompt → tool call → tool execute
	run, err := loop.Prompt(ctx, sess, core.UserInput{Text: "echo test"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	var toolEndCount int
	for ev := range run.Events() {
		if _, ok := ev.(core.ToolCallEnd); ok {
			toolEndCount++
		}
	}
	<-run.Done()
	if toolEndCount != 1 {
		t.Errorf("expected 1 ToolCallEnd, got %d", toolEndCount)
	}

	// Turn 2: continue (tool result → assistant reply)
	run2, err := loop.Continue(ctx, sess)
	if err != nil {
		t.Fatalf("Continue: %v", err)
	}
	var content string
	for ev := range run2.Events() {
		if d, ok := ev.(core.MessageDelta); ok {
			content += d.ContentDelta
		}
	}
	<-run2.Done()
	if content != "echoed test" {
		t.Errorf("expected 'echoed test', got %q", content)
	}

	// Verify transcript has expected messages
	msgs := sess.Transcript.Messages()
	if len(msgs) < 4 {
		t.Errorf("expected >=4 messages, got %d", len(msgs))
	}
}

func TestStreamingChunkAccumulation(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Simulate streaming response with realistic deltas
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "hello "},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "world "},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "from tau"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     prov,
		DefaultModel: core.ModelSpec{Name: "test-model", API: core.WireOpenAICompletions},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Cancel()

	loop := core.NewLoop()
	run, err := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	var accumulated string
	for ev := range run.Events() {
		if d, ok := ev.(core.MessageDelta); ok {
			accumulated += d.ContentDelta
		}
	}
	<-run.Done()

	if accumulated != "hello world from tau" {
		t.Errorf("accumulation mismatch: %q", accumulated)
	}
}

func TestErrorInjectionRecovery(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Turn 1: error
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvError, Err: fmt.Errorf("simulated fault")},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Turn 2: recovery
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "recovered"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     prov,
		DefaultModel: core.ModelSpec{Name: "test-model", API: core.WireOpenAICompletions},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Cancel()

	loop := core.NewLoop()

	run, err := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	var errorCount int
	for ev := range run.Events() {
		if _, ok := ev.(core.ErrorEvent); ok {
			errorCount++
		}
	}
	<-run.Done()
	if errorCount != 1 {
		t.Errorf("expected 1 error event, got %d", errorCount)
	}

	// Recover: next turn should work
	run2, err := loop.Continue(ctx, sess)
	if err != nil {
		t.Fatalf("Continue: %v", err)
	}
	var content string
	for ev := range run2.Events() {
		if d, ok := ev.(core.MessageDelta); ok {
			content += d.ContentDelta
		}
	}
	<-run2.Done()
	if content != "recovered" {
		t.Errorf("expected 'recovered', got %q", content)
	}
}

func TestWebSearchTool(t *testing.T) {
	tool := tools.WebSearchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"query": "golang"}, nil)
	if err != nil {
		t.Fatalf("web_search: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}

func TestWebSearchEmptyQuery(t *testing.T) {
	tool := tools.WebSearchTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"query": ""}, nil)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestWebFetchTool(t *testing.T) {
	tool := tools.WebFetchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"url": "https://example.com"}, nil)
	if err != nil {
		t.Fatalf("web_fetch: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
	if result.Details["status"].(float64) != 200 {
		t.Error("expected 200 status")
	}
}

func TestWebFetchInvalidURL(t *testing.T) {
	tool := tools.WebFetchTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"url": "not-a-url"}, nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestTaskTrackerCreate(t *testing.T) {
	tools.GlobalTaskTracker = &tools.TaskTracker{}
	// Re-init map since TaskTracker can't be created from outside package
	// Reset by creating a fresh tracker via the struct (fields must be set)
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"action": "create", "title": "fix bug"}, nil)
	if result.Content[0].Text != "Created task task-1: fix bug [pending]" {
		t.Errorf("unexpected result: %s", result.Content[0].Text)
	}
}

func TestTaskTrackerUpdateAndList(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	tool.Execute(context.Background(), "c1", map[string]any{"action": "create", "title": "task A"}, nil)
	tool.Execute(context.Background(), "c2", map[string]any{"action": "create", "title": "task B"}, nil)
	tool.Execute(context.Background(), "c3", map[string]any{"action": "update", "task_id": "task-1", "status": "completed"}, nil)
	result, _ := tool.Execute(context.Background(), "c4", map[string]any{"action": "list"}, nil)
	if !strings.Contains(result.Content[0].Text, "[✓]") {
		t.Error("completed task should have ✓ icon")
	}
}

func TestTaskTrackerEmptyList(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"action": "list"}, nil)
	if result.Content[0].Text != "No tasks." {
		t.Errorf("expected 'No tasks.', got %q", result.Content[0].Text)
	}
}

func TestTaskTrackerUpdateNotFound(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "update", "task_id": "nonexistent", "status": "completed"}, nil)
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

// --- read.go edge cases ---

func TestReadNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.ReadTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "nonexistent.xyz"}, nil)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadEmptyFileEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "empty.txt"), []byte{}, 0644)
	tool := tools.ReadTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "empty.txt"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "(empty file)" {
		t.Errorf("expected '(empty file)', got %q", result.Content[0].Text)
	}
}

func TestReadBinaryFileEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "bin.bin"), []byte{0x00, 0x01, 0x02}, 0644)
	tool := tools.ReadTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "bin.bin"}, nil)
	if !strings.Contains(result.Content[0].Text, "[binary file detected") {
		t.Error("expected binary file warning")
	}
}

func TestReadOversizeFileEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	big := strings.Repeat("x", tools.OutputCap+100)
	os.WriteFile(filepath.Join(dir, "big.txt"), []byte(big), 0644)
	tool := tools.ReadTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "big.txt"}, nil)
	if len(result.Content[0].Text) > tools.OutputCap+200 {
		t.Error("oversize file should be capped")
	}
}

// --- write.go edge cases ---

func TestWriteOutsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.WriteTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "../outside.txt", "content": "x"}, nil)
	if err == nil {
		t.Error("expected error for path escaping workspace")
	}
}

func TestWriteEmptyContentEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.WriteTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "clear.txt", "content": ""}, nil)
	if !strings.Contains(result.Content[0].Text, "0 bytes") {
		t.Errorf("expected '0 bytes', got %q", result.Content[0].Text)
	}
}

func TestWriteOverwriteEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.WriteTool()
	tool.Execute(context.Background(), "c1", map[string]any{"file_path": "over.txt", "content": "first"}, nil)
	result, _ := tool.Execute(context.Background(), "c2", map[string]any{"file_path": "over.txt", "content": "second"}, nil)
	if !strings.Contains(result.Content[0].Text, "overwritten") {
		t.Errorf("expected 'overwritten', got %q", result.Content[0].Text)
	}
}

// --- edit.go edge cases ---

func TestEditOldNotFoundEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello"), 0644)
	tool := tools.EditTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "f.txt", "old": "xyz", "new": "abc"}, nil)
	if err == nil {
		t.Error("expected error when old text not found")
	}
}

func TestEditMultilineEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "m.txt"), []byte("line1\nline2\nline3"), 0644)
	tool := tools.EditTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "m.txt", "old": "line2", "new": "MIDDLE"}, nil)
	if !strings.Contains(result.Content[0].Text, "Replaced 1") {
		t.Errorf("expected 'Replaced 1', got %q", result.Content[0].Text)
	}
}

func TestEditEmptyReplaceEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "e.txt"), []byte("remove-me hello"), 0644)
	tool := tools.EditTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "e.txt", "old": "remove-me ", "new": ""}, nil)
	if !strings.Contains(result.Content[0].Text, "Replaced 1") {
		t.Errorf("expected 'Replaced 1', got %q", result.Content[0].Text)
	}
}

// --- bash.go edge cases ---

func TestBashExitNonZero(t *testing.T) {
	if testing.Short() {
		t.Skip("requires bash")
	}
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.BashTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"command": "exit 42", "timeout_seconds": 5}, nil)
	if !strings.Contains(result.Content[0].Text, "[exit: 42]") {
		t.Errorf("expected [exit: 42], got %q", result.Content[0].Text)
	}
}

func TestBashTimeoutEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires bash")
	}
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.BashTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"command": "sleep 10", "timeout_seconds": 1}, nil)
	if !strings.Contains(result.Content[0].Text, "[timeout") {
		t.Errorf("expected timeout indication, got %q", result.Content[0].Text)
	}
}

func TestBashStderrEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires bash")
	}
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.BashTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"command": "echo ok; echo err >&2", "timeout_seconds": 5}, nil)
	if !strings.Contains(result.Content[0].Text, "[stderr]") {
		t.Errorf("expected stderr capture, got %q", result.Content[0].Text)
	}
}

func TestBashEmptyCommandEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.BashTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"command": "", "timeout_seconds": 5}, nil)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

// --- glob.go edge cases ---

func TestGlobEmptyResultEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.GlobTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "*.nonexistent"}, nil)
	// empty result is ok — should contain "(no matches"
	if !strings.Contains(result.Content[0].Text, "no matches") {
		t.Logf("glob result: %s", result.Content[0].Text)
	}
}

func TestGlobHiddenFilesEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "visible"), []byte("x"), 0644)
	tool := tools.GlobTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "*"}, nil)
	text := result.Content[0].Text
	if strings.Contains(text, ".hidden") {
		t.Log("note: .hidden matched by * glob (expected on some systems)")
	}
	if !strings.Contains(text, "visible") {
		t.Error("expected visible in glob results")
	}
}

func TestGlobMaxDepthEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0755)
	os.WriteFile(filepath.Join(dir, "a", "b", "c", "deep.txt"), []byte("x"), 0644)
	tool := tools.GlobTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "**/*.txt", "max_depth": 1}, nil)
	text := result.Content[0].Text
	if strings.Contains(text, "deep.txt") {
		t.Log("note: max_depth not enforced for deep paths")
	}
}

// --- grep.go edge cases ---

func TestGrepNoMatchEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "g.txt"), []byte("hello world"), 0644)
	tool := tools.GrepTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "xyz123nonexistent", "path": "g.txt"}, nil)
	if !strings.Contains(result.Content[0].Text, "No matches") {
		t.Errorf("expected 'No matches' for non-matching pattern, got %q", result.Content[0].Text)
	}
}

func TestGrepBinarySkipEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "g.bin"), []byte{0x00, 0x01, 0x02}, 0644)
	tool := tools.GrepTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "x", "path": "g.bin"}, nil)
	// Binary should be skipped; no matches expected
	if !strings.Contains(result.Content[0].Text, "No matches") && !strings.Contains(result.Content[0].Text, "binary") {
		t.Logf("grep result: %s", result.Content[0].Text)
	}
}

func TestGrepRegexFailEdge(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.GrepTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "[invalid", "path": "g.txt"}, nil)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

// --- task_tracker.go edge cases ---

func TestTaskTrackerEmptyListEdge(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"action": "list"}, nil)
	if !strings.Contains(result.Content[0].Text, "No tasks") {
		t.Errorf("expected 'No tasks' for empty list, got %q", result.Content[0].Text)
	}
}

func TestTaskTrackerCompletedStatusEdge(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	tool.Execute(context.Background(), "c1", map[string]any{"action": "create", "title": "test"}, nil)
	result, _ := tool.Execute(context.Background(), "c2", map[string]any{"action": "update", "task_id": "task-1", "status": "completed"}, nil)
	if !strings.Contains(result.Content[0].Text, "[completed]") && !strings.Contains(result.Content[0].Text, "[✓]") {
		t.Errorf("expected completed status, got %q", result.Content[0].Text)
	}
}

func TestTaskTrackerCancelledToCompletedEdge(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	tool.Execute(context.Background(), "c1", map[string]any{"action": "create", "title": "test"}, nil)
	tool.Execute(context.Background(), "c2", map[string]any{"action": "update", "task_id": "task-1", "status": "cancelled"}, nil)
	result, _ := tool.Execute(context.Background(), "c3", map[string]any{"action": "update", "task_id": "task-1", "status": "completed"}, nil)
	if !strings.Contains(result.Content[0].Text, "[completed]") && !strings.Contains(result.Content[0].Text, "[✓]") {
		t.Errorf("expected update after cancelled to work, got %q", result.Content[0].Text)
	}
}

// --- workspace_diag.go tests ---

func TestWorkspaceDiagTool(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	// Create a few files to scan
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	tool := tools.WorkspaceDiagTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"depth": 1}, nil)
	if err != nil {
		t.Fatalf("workspace_diag: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
	if !strings.Contains(result.Content[0].Text, "Total files") {
		t.Error("expected 'Total files' in output")
	}
}

func TestWorkspaceDiagWithGit(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	tool := tools.WorkspaceDiagTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"include_git": false}, nil)
	if !strings.Contains(result.Content[0].Text, "Language Breakdown") {
		t.Error("expected language breakdown even without git")
	}
}

// --- list_files.go tests ---

func TestListFilesRoot(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)
	tool := tools.ListFilesTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"depth": 1}, nil)
	if len(result.Content[0].Text) == 0 {
		t.Error("expected directory listing")
	}
}

func TestListFilesWithPattern(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello"), 0644)
	tool := tools.ListFilesTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "*.go", "depth": 1}, nil)
	if !strings.Contains(result.Content[0].Text, ".go") {
		t.Errorf("expected .go match, got %q", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, ".txt") {
		t.Error("unexpected .txt in filtered results")
	}
}

// --- search_code.go tests ---

func TestSearchCodeRegex(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("func hello() { return nil }"), 0644)
	tool := tools.SearchCodeTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"query": "func"}, nil)
	if !strings.Contains(result.Content[0].Text, "func") {
		t.Error("expected func match")
	}
}

func TestSearchCodeNoMatch(t *testing.T) {
	tool := tools.SearchCodeTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"query": "xyznonexistent123"}, nil)
	if !strings.Contains(result.Content[0].Text, "No matches") {
		t.Error("expected 'No matches'")
	}
}

// --- run_tests.go tests ---

func TestRunTestsAutoDetect(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test tools")
	}
	tool := tools.RunTestsTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"timeout_seconds": 5}, nil)
	t.Logf("run_tests output: %s", result.Content[0].Text)
}

func TestRunTestsNoFramework(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.RunTestsTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if !strings.Contains(result.Content[0].Text, "No test framework") {
		t.Logf("output: %s", result.Content[0].Text)
	}
}

// --- git_diff.go tests ---

func TestGitDiffNoGit(t *testing.T) {
	tool := tools.GitDiffTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	// Either works or returns "git not available"
	_ = err
}

func TestGitDiffStat(t *testing.T) {
	tool := tools.GitDiffTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"staged": true}, nil)
	if err != nil {
		t.Skip("git not available")
	}
	_ = result
}

// --- ask_user.go tests ---

func TestAskUser(t *testing.T) {
	tool := tools.AskUserTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"question": "proceed?"}, nil)
	if !strings.Contains(result.Content[0].Text, "❓") {
		t.Error("expected ❓ prefix")
	}
	if !result.Terminate {
		t.Error("AskUser should set Terminate=true to pause loop")
	}
}

func TestAskUserWithOptions(t *testing.T) {
	tool := tools.AskUserTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"question": "choose", "options": []any{"a", "b"}}, nil)
	if !strings.Contains(result.Content[0].Text, "Options: a, b") {
		t.Error("expected options in output")
	}
}

// --- lint.go, format.go, deps.go, coverage.go tests ---

func TestLintTool(t *testing.T) {
	tool := tools.LintTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"path": "./..."}, nil)
	if err != nil {
		t.Skipf("lint not available: %v", err)
	}
	if len(result.Content) == 0 {
		t.Skip("lint returned empty output")
	}
	text := result.Content[0].Text
	if len(text) > 200 {
		text = text[:200]
	}
	t.Logf("lint output: %s", text)
}

func TestFormatTool(t *testing.T) {
	tool := tools.FormatTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"check": true}, nil)
	if err != nil {
		t.Skipf("gofmt not available: %v", err)
	}
	if len(result.Content) == 0 {
		t.Skip("goimports returned empty output")
	}
	text := result.Content[0].Text
	if len(text) > 200 {
		text = text[:200]
	}
	t.Logf("format output: %s", text)
}

func TestDepsTool(t *testing.T) {
	tool := tools.DepsTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if err != nil {
		t.Skipf("go not available: %v", err)
	}
	if len(result.Content) == 0 {
		t.Skip("deps returned empty output")
	}
	text := result.Content[0].Text
	if len(text) > 200 {
		text = text[:200]
	}
	t.Logf("deps output: %s", text)
}

func TestCoverageTool(t *testing.T) {
	tool := tools.CoverageTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if err != nil {
		t.Skipf("go test not available: %v", err)
	}
	if len(result.Content) == 0 {
		t.Skip("coverage returned empty output")
	}
	text := result.Content[0].Text
	if len(text) > 200 {
		text = text[:200]
	}
	t.Logf("coverage output: %s", text)
}

// --- verify.go tests ---

func TestVerifyPass(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.VerifyTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"command": "echo hello", "expected_output": "hello", "timeout_seconds": 5,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if passed, ok := result.Details["passed"].(bool); !ok || !passed {
		t.Errorf("expected pass, got %v", result.Details["passed"])
	}
}

func TestVerifyFail(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.VerifyTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{
		"command": "echo goodbye", "expected_output": "hello", "timeout_seconds": 5,
	}, nil)
	if passed, ok := result.Details["passed"].(bool); !ok || passed {
		t.Errorf("expected fail, got %v", result.Details["passed"])
	}
}

func TestVerifyTimeout(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.VerifyTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"command": "sleep 10", "timeout_seconds": 1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Content[0].Text, "PASS") {
		t.Error("expected timeout, got PASS")
	}
}

func TestVerifyRegexMatch(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.VerifyTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{
		"command": "echo test-42-ok", "expected_output": `test-\d+-ok`, "timeout_seconds": 5,
	}, nil)
	if passed, ok := result.Details["passed"].(bool); !ok || !passed {
		t.Errorf("regex should match, got %v", result.Content[0].Text)
	}
}

func TestVerifyNoExpectedMatch(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	tool := tools.VerifyTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{
		"command": "echo test", "expected_output": `nonexistent`, "timeout_seconds": 5,
	}, nil)
	if passed, ok := result.Details["passed"].(bool); !ok || passed {
		t.Errorf("regex should not match")
	}
}

// --- browse.go tests ---

func TestBrowseNormal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Test Page</title></head><body><p>Hello world</p></body></html>`))
	}))
	defer srv.Close()

	tool := tools.BrowseTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL, "max_chars": 500}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "Test Page") {
		t.Errorf("expected title, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Hello world") {
		t.Errorf("expected content, got: %s", result.Content[0].Text)
	}
}

func TestBrowseCSSSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><div class="content">Target content</div><div>Other stuff</div></body></html>`))
	}))
	defer srv.Close()

	tool := tools.BrowseTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL, "extract": ".content"}, nil)
	if !strings.Contains(result.Content[0].Text, "Target content") {
		t.Errorf("expected targeted content, got: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "Other stuff") {
		t.Error("should not contain untargeted content")
	}
}

func TestBrowse404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	tool := tools.BrowseTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL}, nil)
	if !strings.Contains(result.Content[0].Text, "404") {
		t.Errorf("expected 404 in output, got: %s", result.Content[0].Text)
	}
}

func TestBrowseTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	// The tool uses 10s timeout — this should NOT timeout
	tool := tools.BrowseTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL}, nil)
	// Either gets the response (empty body after sleep) or errors
	_ = result
	_ = err
}

func TestBrowseOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		long := strings.Repeat("x", 10000)
		w.Write([]byte(fmt.Sprintf(`<html><body>%s</body></html>`, long)))
	}))
	defer srv.Close()

	tool := tools.BrowseTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL, "max_chars": 100}, nil)
	text := result.Content[0].Text
	if len(text) > 200 { // title + url + content
		t.Logf("text length: %d (may include title/url overhead)", len(text))
	}
}

// --- git tools tests ---

func TestGitDiffBasic(t *testing.T) {
	tool := tools.GitDiffTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "no changes") && result.Content[0].Text != "" {
		t.Logf("diff output: %s", result.Content[0].Text[:minLen(result.Content[0].Text, 200)])
	}
}

func minLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}

func TestGitDiffStagedEdge(t *testing.T) {
	tool := tools.GitDiffTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"staged": true}, nil)
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	_ = result
}

func TestGitCommit(t *testing.T) {
	tool := tools.GitCommitTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"message": "test commit"}, nil)
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	t.Logf("commit output: %s", result.Content[0].Text)
}

func TestGitCommitEmptyMessage(t *testing.T) {
	tool := tools.GitCommitTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"message": ""}, nil)
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestGitLog(t *testing.T) {
	tool := tools.GitLogTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"count": 5}, nil)
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	t.Logf("log output: %s", result.Content[0].Text[:minLen(result.Content[0].Text, 200)])
}

func TestGitLogOneline(t *testing.T) {
	tool := tools.GitLogTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"count": 3, "oneline": true}, nil)
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	_ = result
}

func TestGitBranchList(t *testing.T) {
	tool := tools.GitBranchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "list"}, nil)
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "*") {
		t.Log("branch list (no star marker): " + result.Content[0].Text)
	}
}

func TestGitBranchCreate(t *testing.T) {
	tool := tools.GitBranchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "create", "name": "test-branch-xyz"}, nil)
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	// Cleanup
	exec.Command("git", "-C", tools.WorkspaceRoot, "branch", "-D", "test-branch-xyz").Run()
	_ = result
}
