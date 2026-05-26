package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplates(t *testing.T) {
	dir := t.TempDir()
	content := `# name: code-review
# version: 1.0.0
# description: Review code for bugs and style
# category: review
# variables: language, focus
You are reviewing {{.language}} code. Focus on {{.focus}}.`
	os.WriteFile(filepath.Join(dir, "code-review.tmpl"), []byte(content), 0644)

	tmpls, err := LoadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tmpls) != 1 {
		t.Fatalf("expected 1 template, got %d", len(tmpls))
	}
	tpl := tmpls["code-review"]
	if tpl.Category != "review" {
		t.Errorf("expected category 'review', got %q", tpl.Category)
	}
	if len(tpl.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(tpl.Variables))
	}
}

func TestRenderTemplate(t *testing.T) {
	tpl := &PromptTemplate{
		Name: "test", Version: "1.0", Content: "Hello {{.name}}!",
		Variables: []string{"name"},
	}
	result, err := tpl.Render(map[string]string{"name": "tau"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello tau!" {
		t.Errorf("expected 'Hello tau!', got %q", result)
	}
}

func TestRenderMissingVar(t *testing.T) {
	tpl := &PromptTemplate{
		Name: "test", Version: "1.0", Content: "Hello {{.name}}!",
		Variables: []string{"name"},
	}
	_, err := tpl.Render(map[string]string{})
	if err == nil {
		t.Error("expected error for missing variable")
	}
}

func TestValidateTemplate(t *testing.T) {
	tpl := &PromptTemplate{Name: "test", Content: "Hello {{.name}}!"}
	if err := tpl.Validate(); err != nil {
		t.Errorf("valid template failed: %v", err)
	}

	tpl2 := &PromptTemplate{Name: "bad", Content: "Hello {{.name}"}
	if err := tpl2.Validate(); err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestListTemplates(t *testing.T) {
	tmpls := map[string]*PromptTemplate{
		"a": {Name: "a", Category: "review"},
		"b": {Name: "b", Category: "testing"},
		"c": {Name: "c", Category: "review"},
	}
	reviews := ListTemplates(tmpls, "review")
	if len(reviews) != 2 {
		t.Errorf("expected 2 review templates, got %d", len(reviews))
	}

	all := ListTemplates(tmpls, "")
	if len(all) != 3 {
		t.Errorf("expected 3 all templates, got %d", len(all))
	}
}
