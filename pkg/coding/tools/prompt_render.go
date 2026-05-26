package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akzj/tau/core"
)

// promptRenderSchema adapts json.RawMessage to core.ToolSchema.
type promptRenderSchema struct {
	raw json.RawMessage
}

func (s promptRenderSchema) Marshal() (json.RawMessage, error)      { return s.raw, nil }
func (s promptRenderSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// PromptRenderTool creates a tool for rendering prompt templates.
func PromptRenderTool() core.Tool {
	raw := json.RawMessage(`{
		"type": "object",
		"properties": {
			"template": {"type": "string", "description": "Template name to render (or 'list' to see all)"},
			"vars": {"type": "object", "description": "Variable values as key-value map"}
		},
		"required": ["template"]
	}`)

	templatesDir := findPromptDir()

	return core.Tool{
		Name:        "prompt_render",
		Description: "Render a structured prompt template with variables. Use template='list' to see available templates.",
		Schema:      promptRenderSchema{raw: raw},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Template string            `json:"template"`
				Vars     map[string]string `json:"vars"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.Template == "list" {
				return listTemplates(templatesDir)
			}

			tmpls, err := core.LoadTemplates(templatesDir)
			if err != nil {
				return core.ToolResult{}, err
			}

			tpl, ok := tmpls[args.Template]
			if !ok {
				var names []string
				for n := range tmpls {
					names = append(names, n)
				}
				sort.Strings(names)
				return core.ToolResult{}, fmt.Errorf("template %q not found. Available: %s", args.Template, strings.Join(names, ", "))
			}

			result, err := tpl.Render(args.Vars)
			if err != nil {
				return core.ToolResult{}, err
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: result}},
				Details: map[string]any{"template": args.Template, "version": tpl.Version},
			}, nil
		},
	}
}

func findPromptDir() string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(WorkspaceRoot, ".tau", "prompts"),
		filepath.Join(home, ".tau", "prompts"),
		"pkg/coding/skills/prompts",
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			return d
		}
	}
	return filepath.Join(home, ".tau", "prompts")
}

func listTemplates(dir string) (core.ToolResult, error) {
	tmpls, err := core.LoadTemplates(dir)
	if err != nil {
		return core.ToolResult{}, err
	}
	if len(tmpls) == 0 {
		return core.ToolResult{Content: []core.Content{{Type: "text", Text: "No templates found."}}}, nil
	}
	var lines []string
	for name, t := range tmpls {
		lines = append(lines, fmt.Sprintf("- **%s** (v%s): %s [category: %s]", name, t.Version, t.Description, t.Category))
	}
	sort.Strings(lines)
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: "## Prompt Templates\n\n" + strings.Join(lines, "\n")}},
	}, nil
}
