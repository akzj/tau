package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// PromptTemplate is a structured, reusable prompt with metadata.
type PromptTemplate struct {
	Name        string   // unique identifier (e.g., "code-review")
	Version     string   // semver (e.g., "1.0.0")
	Description string   // what this template is for
	Content     string   // the template body (text/template syntax)
	Variables   []string // required variable names
	Category    string   // grouping: "review", "refactor", "testing", "docs", "general"
}

// LoadTemplates scans a directory for .tmpl files and parses them.
func LoadTemplates(dir string) (map[string]*PromptTemplate, error) {
	templates := make(map[string]*PromptTemplate)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return templates, nil
		}
		return nil, fmt.Errorf("read dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".tmpl" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		tmpl := parseTemplate(string(data))
		if tmpl.Name == "" {
			tmpl.Name = strings.TrimSuffix(e.Name(), ".tmpl")
		}
		templates[tmpl.Name] = tmpl
	}
	return templates, nil
}

// parseTemplate extracts frontmatter and content from a .tmpl file.
func parseTemplate(raw string) *PromptTemplate {
	t := &PromptTemplate{}
	lines := strings.Split(raw, "\n")
	i := 0
	for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
		line := strings.TrimPrefix(strings.TrimSpace(lines[i]), "# ")
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			switch key {
			case "name":
				t.Name = val
			case "version":
				t.Version = val
			case "description":
				t.Description = val
			case "category":
				t.Category = val
			case "variables":
				for _, v := range strings.Split(val, ",") {
					t.Variables = append(t.Variables, strings.TrimSpace(v))
				}
			}
		}
		i++
	}
	t.Content = strings.TrimSpace(strings.Join(lines[i:], "\n"))
	if t.Version == "" {
		t.Version = "1.0.0"
	}
	return t
}

// Render renders a template with the given variable values.
func (t *PromptTemplate) Render(vars map[string]string) (string, error) {
	for _, req := range t.Variables {
		if _, ok := vars[req]; !ok {
			return "", fmt.Errorf("missing required variable: %s", req)
		}
	}
	tmpl, err := template.New(t.Name).Parse(t.Content)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

// Validate checks if a template can be parsed successfully.
func (t *PromptTemplate) Validate() error {
	_, err := template.New(t.Name).Parse(t.Content)
	return err
}

// ListTemplates filters templates by category. Empty category = all.
func ListTemplates(templates map[string]*PromptTemplate, category string) []*PromptTemplate {
	var result []*PromptTemplate
	for _, t := range templates {
		if category == "" || t.Category == category {
			result = append(result, t)
		}
	}
	return result
}
