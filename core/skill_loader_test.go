package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillLoaderLoad(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test-skill.md"), []byte(`---
name: "code-review"
description: "Review code for bugs"
---
Check for bugs.`), 0644)

	sl := NewSkillLoader(dir)
	if len(sl.skills) == 0 {
		t.Error("expected at least 1 skill")
	}
}

func TestSkillLoaderMatchByTool(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "review.md"), []byte(`---
name: "code-review"
description: "Review code for bugs and style issues"
when_to_use: "When reviewing code changes"
---
Check for bugs and style issues.`), 0644)
	os.WriteFile(filepath.Join(dir, "debug.md"), []byte(`---
name: "debugging"
description: "Debug runtime errors systematically"
when_to_use: "When encountering unexpected errors"
---
Isolate, reproduce, fix.`), 0644)

	sl := NewSkillLoader(dir)
	results := sl.MatchSkills([]string{"read", "grep"}, "review the code for bugs", 5, 200)
	if len(results) == 0 {
		t.Error("expected matched skills")
	}
	if results[0].Name != "code-review" {
		t.Errorf("expected code-review first, got %s", results[0].Name)
	}
}

func TestSkillLoaderDedup(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte(`---
name: "duplicate"
description: "First"
---
Body 1`), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte(`---
name: "duplicate"
description: "Second"
---
Body 2`), 0644)

	sl := NewSkillLoader(dir)
	results := sl.MatchSkills([]string{"test"}, "duplicate", 5, 200)
	count := 0
	for _, r := range results {
		if r.Name == "duplicate" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected ≤1 duplicate skill, got %d", count)
	}
}

func TestSkillLoaderTokenLimit(t *testing.T) {
	dir := t.TempDir()
	longContent := "x"
	for i := 0; i < 500; i++ {
		longContent += "x"
	}
	os.WriteFile(filepath.Join(dir, "long.md"), []byte("---\nname: \"long-skill\"\ndescription: \"Long\"\n---\n"+longContent), 0644)

	sl := NewSkillLoader(dir)
	results := sl.MatchSkills([]string{"test"}, "long", 5, 100)
	if len(results) > 0 {
		for _, r := range results {
			if len(r.Content) > 103 {
				t.Errorf("content exceeded maxChars: %d", len(r.Content))
			}
		}
	}
}
