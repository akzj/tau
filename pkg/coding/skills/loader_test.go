package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	l := NewLoader([]string{"/tmp"})
	if l == nil {
		t.Error("NewLoader returned nil")
	}
}

func TestLoaderEmptyDir(t *testing.T) {
	dir := t.TempDir()
	l := NewLoader([]string{dir})
	skills, err := l.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills from empty dir, got %d", len(skills))
	}
}

func TestLoaderNonexistentDir(t *testing.T) {
	l := NewLoader([]string{"/nonexistent/dir/path"})
	skills, err := l.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestParseSkillFile(t *testing.T) {
	content := `---
name: "test-skill"
description: "A test skill"
disable-model-invocation: false
---
## Guidelines
- Do this
- Avoid that
`
	skill := parseSkillFile(content, "/tmp/test-skill.md")
	if skill.Name != "test-skill" {
		t.Errorf("expected 'test-skill', got %q", skill.Name)
	}
	if skill.Description != "A test skill" {
		t.Errorf("expected 'A test skill', got %q", skill.Description)
	}
	if skill.DisableInvocation {
		t.Error("expected disable-model-invocation to be false")
	}
}

func TestParseSkillFileNoFrontmatter(t *testing.T) {
	content := `## Just some content
No frontmatter here.
`
	skill := parseSkillFile(content, "/path/to/my-skill.md")
	if skill.Name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", skill.Name)
	}
	if skill.Description != "my-skill" {
		t.Errorf("expected fallback description 'my-skill', got %q", skill.Description)
	}
}

func TestParseSkillFileDisableInvocationTrue(t *testing.T) {
	content := `---
name: "private-skill"
disable-model-invocation: true
---
Content
`
	skill := parseSkillFile(content, "/tmp/private.md")
	if !skill.DisableInvocation {
		t.Error("expected DisableInvocation=true")
	}
}

func TestLoaderLoadRealFile(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: "loaded-skill"
description: "Loaded from disk"
---
## Content
`
	os.WriteFile(filepath.Join(dir, "test.md"), []byte(content), 0644)

	l := NewLoader([]string{dir})
	skills, err := l.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "loaded-skill" {
		t.Errorf("name mismatch: %q", skills[0].Name)
	}
}

func TestLoaderOverride(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// First dir has v1
	os.WriteFile(filepath.Join(dir1, "override.md"), []byte(`---
name: "override-skill"
description: "v1"
---
v1 body
`), 0644)

	// Second dir has v2 (should override)
	os.WriteFile(filepath.Join(dir2, "override.md"), []byte(`---
name: "override-skill"
description: "v2"
---
v2 body
`), 0644)

	l := NewLoader([]string{dir1, dir2})
	skills, err := l.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill after override, got %d", len(skills))
	}
	if skills[0].Description != "v2" {
		t.Errorf("expected v2 (later overrides earlier), got %q", skills[0].Description)
	}
}
