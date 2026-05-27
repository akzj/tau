package skills

import (
	"strings"
	"testing"
)

// ── Registry CRUD ──

func TestNewSkillRegistry(t *testing.T) {
	r := NewSkillRegistry()
	if r == nil {
		t.Fatal("NewSkillRegistry returned nil")
	}
	if r.Count() != 0 {
		t.Errorf("expected 0 skills, got %d", r.Count())
	}
}

func TestGlobalSkillsNotEmpty(t *testing.T) {
	if GlobalSkills.Count() < 6 {
		t.Errorf("expected at least 6 skills, got %d", GlobalSkills.Count())
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewSkillRegistry()
	r.Register(&SkillDefinition{
		Name:        "test_skill",
		Description: "A test",
		Tools:       []string{"read", "write"},
		Prompt:      "Do stuff.",
		Category:    "test",
	})
	s, ok := r.Get("test_skill")
	if !ok {
		t.Fatal("expected test_skill to be found")
	}
	if s.Name != "test_skill" {
		t.Errorf("name mismatch: %q", s.Name)
	}
	if s.Description != "A test" {
		t.Errorf("description mismatch: %q", s.Description)
	}
	if len(s.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(s.Tools))
	}
}

func TestGetMissingSkill(t *testing.T) {
	r := NewSkillRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected false for missing skill")
	}
}

func TestListSorted(t *testing.T) {
	r := NewSkillRegistry()
	r.Register(&SkillDefinition{Name: "zebra", Description: "z", Category: "a"})
	r.Register(&SkillDefinition{Name: "alpha", Description: "a", Category: "a"})
	r.Register(&SkillDefinition{Name: "mike", Description: "m", Category: "a"})
	skills := r.List()
	if len(skills) != 3 {
		t.Fatalf("expected 3, got %d", len(skills))
	}
	if skills[0].Name != "alpha" || skills[1].Name != "mike" || skills[2].Name != "zebra" {
		t.Errorf("not sorted: %v", []string{skills[0].Name, skills[1].Name, skills[2].Name})
	}
}

func TestCount(t *testing.T) {
	r := NewSkillRegistry()
	if r.Count() != 0 {
		t.Errorf("expected 0, got %d", r.Count())
	}
	r.Register(&SkillDefinition{Name: "one", Category: "x"})
	r.Register(&SkillDefinition{Name: "two", Category: "x"})
	if r.Count() != 2 {
		t.Errorf("expected 2, got %d", r.Count())
	}
}

func TestDuplicateRegistration(t *testing.T) {
	r := NewSkillRegistry()
	r.Register(&SkillDefinition{Name: "dup", Description: "v1", Category: "x"})
	r.Register(&SkillDefinition{Name: "dup", Description: "v2", Category: "x"})
	s, ok := r.Get("dup")
	if !ok {
		t.Fatal("dup not found")
	}
	if s.Description != "v2" {
		t.Errorf("expected v2 (override), got %q", s.Description)
	}
}

// ── Each skill registration ──

func TestCodeReviewSkillRegistered(t *testing.T) {
	s, ok := GlobalSkills.Get("code_review")
	if !ok {
		t.Fatal("code_review not registered")
	}
	if s.Category != "quality" {
		t.Errorf("expected category quality, got %q", s.Category)
	}
	if len(s.Tools) == 0 {
		t.Error("code_review has no tools")
	}
	if s.Prompt == "" {
		t.Error("code_review has no prompt")
	}
}

func TestDebugSessionSkillRegistered(t *testing.T) {
	s, ok := GlobalSkills.Get("debug_session")
	if !ok {
		t.Fatal("debug_session not registered")
	}
	if s.Category != "debug" {
		t.Errorf("expected category debug, got %q", s.Category)
	}
	if len(s.Tools) == 0 {
		t.Error("debug_session has no tools")
	}
}

func TestRefactorModuleSkillRegistered(t *testing.T) {
	s, ok := GlobalSkills.Get("refactor_module")
	if !ok {
		t.Fatal("refactor_module not registered")
	}
	if s.Category != "refactor" {
		t.Errorf("expected category refactor, got %q", s.Category)
	}
	if len(s.Tools) == 0 {
		t.Error("refactor_module has no tools")
	}
}

func TestOnboardNewcomerSkillRegistered(t *testing.T) {
	s, ok := GlobalSkills.Get("onboard_newcomer")
	if !ok {
		t.Fatal("onboard_newcomer not registered")
	}
	if s.Category != "onboarding" {
		t.Errorf("expected category onboarding, got %q", s.Category)
	}
	if len(s.Tools) == 0 {
		t.Error("onboard_newcomer has no tools")
	}
}

func TestSecurityAuditSkillRegistered(t *testing.T) {
	s, ok := GlobalSkills.Get("security_audit")
	if !ok {
		t.Fatal("security_audit not registered")
	}
	if s.Category != "security" {
		t.Errorf("expected category security, got %q", s.Category)
	}
	if len(s.Tools) == 0 {
		t.Error("security_audit has no tools")
	}
}

func TestPerfProfileSkillRegistered(t *testing.T) {
	s, ok := GlobalSkills.Get("perf_profile")
	if !ok {
		t.Fatal("perf_profile not registered")
	}
	if s.Category != "performance" {
		t.Errorf("expected category performance, got %q", s.Category)
	}
	if len(s.Tools) == 0 {
		t.Error("perf_profile has no tools")
	}
}

// ── BuildPrompt ──

func TestBuildPromptValid(t *testing.T) {
	prompt, err := GlobalSkills.BuildPrompt("code_review", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "code_review") {
		t.Error("prompt does not contain skill name")
	}
	if !strings.Contains(prompt, "Recommended Tool Sequence") {
		t.Error("prompt missing tool sequence header")
	}
}

func TestBuildPromptWithContext(t *testing.T) {
	prompt, err := GlobalSkills.BuildPrompt("debug_session", "file: main.go, error: nil pointer")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "main.go") {
		t.Error("prompt missing context")
	}
	if !strings.Contains(prompt, "nil pointer") {
		t.Error("prompt missing context detail")
	}
}

func TestBuildPromptUnknownSkill(t *testing.T) {
	_, err := GlobalSkills.BuildPrompt("nonexistent_skill", "")
	if err == nil {
		t.Error("expected error for unknown skill")
	}
}

func TestBuildPromptIncludesToolSequence(t *testing.T) {
	prompt, err := GlobalSkills.BuildPrompt("refactor_module", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"run_test", "find_references", "format_code"} {
		if !strings.Contains(prompt, tool) {
			t.Errorf("prompt missing tool: %s", tool)
		}
	}
}

func TestBuildPromptAllSixSkills(t *testing.T) {
	names := []string{
		"code_review", "debug_session", "refactor_module",
		"onboard_newcomer", "security_audit", "perf_profile",
	}
	for _, name := range names {
		prompt, err := GlobalSkills.BuildPrompt(name, "")
		if err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
			continue
		}
		if prompt == "" {
			t.Errorf("%s: returned empty prompt", name)
		}
		if !strings.Contains(prompt, name) {
			t.Errorf("%s: prompt missing skill name", name)
		}
	}
}

func TestBuildPromptEmptyContext(t *testing.T) {
	prompt, err := GlobalSkills.BuildPrompt("security_audit", "")
	if err != nil {
		t.Fatal(err)
	}
	// Should not contain "### Context:" header when context is empty
	if strings.Contains(prompt, "### Context:\n\n") {
		t.Error("prompt has empty context section")
	}
}

// ── Tool sequence validation ──

func TestCodeReviewTools(t *testing.T) {
	s, _ := GlobalSkills.Get("code_review")
	required := []string{"search_code", "find_references", "lint_code"}
	for _, want := range required {
		found := false
		for _, got := range s.Tools {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("code_review missing required tool: %s", want)
		}
	}
}

func TestDebugSessionTools(t *testing.T) {
	s, _ := GlobalSkills.Get("debug_session")
	required := []string{"run_test", "search_code", "read"}
	for _, want := range required {
		found := false
		for _, got := range s.Tools {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("debug_session missing required tool: %s", want)
		}
	}
}

func TestRefactorModuleTools(t *testing.T) {
	s, _ := GlobalSkills.Get("refactor_module")
	required := []string{"run_test", "find_references", "format_code"}
	for _, want := range required {
		found := false
		for _, got := range s.Tools {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("refactor_module missing required tool: %s", want)
		}
	}
}

func TestOnboardNewcomerTools(t *testing.T) {
	s, _ := GlobalSkills.Get("onboard_newcomer")
	required := []string{"read", "search_code", "browse"}
	for _, want := range required {
		found := false
		for _, got := range s.Tools {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("onboard_newcomer missing required tool: %s", want)
		}
	}
}

func TestSecurityAuditTools(t *testing.T) {
	s, _ := GlobalSkills.Get("security_audit")
	required := []string{"search_code", "lint_code", "env_manage"}
	for _, want := range required {
		found := false
		for _, got := range s.Tools {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("security_audit missing required tool: %s", want)
		}
	}
}

func TestPerfProfileTools(t *testing.T) {
	s, _ := GlobalSkills.Get("perf_profile")
	required := []string{"run_bench", "search_code", "read"}
	for _, want := range required {
		found := false
		for _, got := range s.Tools {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("perf_profile missing required tool: %s", want)
		}
	}
}

// ── Edge cases ──

func TestListEmptyRegistry(t *testing.T) {
	r := NewSkillRegistry()
	skills := r.List()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills from empty registry, got %d", len(skills))
	}
}

func TestEmptyRegistryCount(t *testing.T) {
	r := NewSkillRegistry()
	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}
}

func TestGlobalSkillsListHasAllSix(t *testing.T) {
	skills := GlobalSkills.List()
	if len(skills) < 6 {
		t.Fatalf("expected at least 6 skills, got %d", len(skills))
	}
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	required := []string{
		"code_review", "debug_session", "refactor_module",
		"onboard_newcomer", "security_audit", "perf_profile",
	}
	for _, name := range required {
		if !names[name] {
			t.Errorf("GlobalSkills.List() missing: %s", name)
		}
	}
}

func TestSkillDefinitionFields(t *testing.T) {
	s, _ := GlobalSkills.Get("code_review")
	if s.Name == "" {
		t.Error("Name is empty")
	}
	if s.Description == "" {
		t.Error("Description is empty")
	}
	if s.Category == "" {
		t.Error("Category is empty")
	}
	if len(s.Tools) == 0 {
		t.Error("Tools is empty")
	}
	if s.Prompt == "" {
		t.Error("Prompt is empty")
	}
}

func TestBuildPromptContainsDescription(t *testing.T) {
	s, _ := GlobalSkills.Get("perf_profile")
	prompt, err := GlobalSkills.BuildPrompt("perf_profile", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, s.Description) {
		t.Error("prompt missing description")
	}
}