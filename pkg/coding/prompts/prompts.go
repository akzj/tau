package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/akzj/tau/core"
)

//go:embed system_base.md dynamic.md
var promptFS embed.FS

// piVarPattern matches $1, $2, ..., $@, $ARGUMENTS
var piVarPattern = regexp.MustCompile(`\$\d+|\$@|\$ARGUMENTS`)

// Input carries all data needed to build the system prompt.
type Input struct {
	WorkspaceRoot string
	Skills        []SkillInfo
	Tools         []core.Tool
}

// SkillInfo is a minimal skill reference for prompt rendering.
type SkillInfo struct {
	Name        string
	Description string
}

// BuildSystemPrompt renders the system_base.md template.
func BuildSystemPrompt(in Input) (string, error) {
	tmpl, err := template.ParseFS(promptFS, "system_base.md")
	if err != nil {
		return "", fmt.Errorf("parse system_base.md: %w", err)
	}

	// Build skills block
	skillsBlock := ""
	if len(in.Skills) > 0 {
		var lines []string
		for _, s := range in.Skills {
			lines = append(lines, fmt.Sprintf("- **%s**: %s", s.Name, s.Description))
		}
		skillsBlock = "## AVAILABLE SKILLS\n" + strings.Join(lines, "\n") +
			"\n\nTo invoke a skill, say \"use skill <name>\"."
	}

	// Build tools block
	var toolLines []string
	for _, t := range in.Tools {
		toolLines = append(toolLines, fmt.Sprintf("- **%s**: %s", t.Name, t.Description))
	}
	toolsBlock := "## AVAILABLE TOOLS\n" + strings.Join(toolLines, "\n")

	data := map[string]string{
		"WorkspaceRoot": in.WorkspaceRoot,
		"SkillsBlock":   skillsBlock,
		"ToolsBlock":    toolsBlock,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// BuildDynamicContext renders the dynamic.md template.
func BuildDynamicContext(in DynamicInput) (string, error) {
	tmpl, err := template.ParseFS(promptFS, "dynamic.md")
	if err != nil {
		return "", fmt.Errorf("parse dynamic.md: %w", err)
	}

	data := map[string]string{
		"GitStatus":    in.GitStatus,
		"WorkingFiles": in.WorkingFiles,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// DynamicInput carries per-turn dynamic context.
type DynamicInput struct {
	GitStatus    string
	WorkingFiles string
}

// RenderPrompt renders a template with auto-detection of pi-style ($1/$@) vs text/template syntax.
func RenderPrompt(tmpl string, args []string) string {
	if piVarPattern.MatchString(tmpl) {
		return renderPiStyle(tmpl, args)
	}
	return tmpl
}

// renderPiStyle substitutes $1, $2, ..., $@, and $ARGUMENTS in a prompt template.
func renderPiStyle(tmpl string, args []string) string {
	result := strings.ReplaceAll(tmpl, "$@", strings.Join(args, " "))
	result = strings.ReplaceAll(result, "$ARGUMENTS", strings.Join(args, " "))
	for i, arg := range args {
		result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i+1), arg)
	}
	return result
}