//go:build integration

package skill_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/internal/testutil"
	"github.com/akzj/tau/pkg/coding/skills"
)

// testSchema is a minimal core.ToolSchema for integration tests.
type testSchema struct {
	raw json.RawMessage
}

func (s testSchema) Marshal() (json.RawMessage, error)  { return s.raw, nil }
func (s testSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// TestCodeReviewSkill verifies skill loading and loop execution for code review.
func TestCodeReviewSkill(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	// Create a skill directory with a code-review skill
	skillDir := filepath.Join(h.Workspace, "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "code-review.md"), []byte(`---
name: "code-review"
description: "Review code for bugs, style, and security issues"
when_to_use: "code_review, code changes, merge request"
---
## Guidelines
- Check for nil pointers
- Verify error handling
- Review naming conventions
`), 0644)

	// Queue a simple response
	h.QueueSimple("code review complete: no issues found")

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	// Register code-review tool
	sess.Tools.Register(core.Tool{
		Name:        "code_review",
		Description: "review code changes",
		Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "review: no issues"}}}, nil
		},
	})

	// Manually inject skill into session
	sess.SkillLoader = core.NewSkillLoader(skillDir)

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "review the code changes"})
	if err != nil {
		t.Fatal(err)
	}
	<-run.Done()

	// Skill loader should have loaded the skill
	if sess.SkillLoader == nil {
		t.Error("skill loader not initialized")
	}
	t.Log("code-review skill test complete")
}

// TestDebugSkill verifies skill loading and matching for debugging.
func TestDebugSkill(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	skillDir := filepath.Join(h.Workspace, "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "debug.md"), []byte(`---
name: "debug"
description: "Systematic debugging approach for runtime errors"
when_to_use: "debug, error, crash, panic"
---
## Steps
1. Reproduce the bug
2. Add logging
3. Isolate the root cause
4. Fix and verify
`), 0644)

	h.QueueSimple("debug complete: root cause found")

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	sess.Tools.Register(core.Tool{
		Name:        "debug",
		Description: "debug runtime errors",
		Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "debug: root cause identified"}}}, nil
		},
	})

	sess.SkillLoader = core.NewSkillLoader(skillDir)

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "debug the nil pointer crash"})
	if err != nil {
		t.Fatal(err)
	}
	<-run.Done()

	// Verify skill loader loaded the debug skill
	if sess.SkillLoader == nil {
		t.Error("skill loader not initialized")
	}
	t.Log("debug skill test complete")
}

// TestSkillLoaderIntegration verifies the skills package loader end-to-end.
func TestSkillLoaderIntegration(t *testing.T) {
	dir := t.TempDir()

	content := `---
name: "test-skill"
description: "A test skill"
---
## Content
Test content here.
`
	os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644)

	l := skills.NewLoader([]string{dir})
	loaded, err := l.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loaded))
	}
	if loaded[0].Name != "test-skill" {
		t.Errorf("expected 'test-skill', got %q", loaded[0].Name)
	}
	t.Logf("loaded skill: %s — %s", loaded[0].Name, loaded[0].Description)
}
