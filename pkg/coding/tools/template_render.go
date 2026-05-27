package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/akzj/tau/core"
)

// TemplateRenderTool creates a Go template rendering tool.
//
// Parameters:
//
//	template (string, required) — Go template string or path to template file
//	vars     (string, optional) — JSON variables for template
//	format   (string, optional, default: text) — text | html
//
// Uses text/template or html/template based on format.
func TemplateRenderTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"template": {"type": "string", "description": "Go template string or path to .tmpl file"},
			"vars": {"type": "string", "description": "JSON object with template variables"},
			"format": {"type": "string", "description": "Output format: text (default) or html"}
		},
		"required": ["template"]
	}`)

	return core.Tool{
		Name:        "template_render",
		Description: "Render Go templates (text or html) with JSON variable input.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Template string `json:"template"`
				Vars     string `json:"vars"`
				Format   string `json:"format"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Template == "" {
				return core.ToolResult{}, fmt.Errorf("template required")
			}
			if args.Format == "" {
				args.Format = "text"
			}

			// Parse template variables
			var tmplVars any
			if args.Vars != "" {
				if err := json.Unmarshal([]byte(args.Vars), &tmplVars); err != nil {
					return core.ToolResult{}, fmt.Errorf("invalid vars JSON: %w", err)
				}
			} else {
				tmplVars = map[string]any{}
			}

			// Determine if template is inline or file path
			tmplContent := args.Template
			if strings.HasSuffix(args.Template, ".tmpl") || strings.HasSuffix(args.Template, ".gotmpl") {
				resolved, err := ResolvePath(args.Template)
				if err == nil {
					data, err := os.ReadFile(resolved)
					if err != nil {
						return core.ToolResult{}, fmt.Errorf("read template file: %w", err)
					}
					tmplContent = string(data)
				}
			}

			var output string
			switch args.Format {
			case "html":
				// Use html/template
				tmpl, err := template.New("tmpl").Option("missingkey=error").Parse(tmplContent)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("parse template: %w", err)
				}
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, tmplVars); err != nil {
					return core.ToolResult{}, fmt.Errorf("execute template: %w", err)
				}
				output = buf.String()
			default: // text
				tmpl, err := template.New("tmpl").Option("missingkey=error").Parse(tmplContent)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("parse template: %w", err)
				}
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, tmplVars); err != nil {
					return core.ToolResult{}, fmt.Errorf("execute template: %w", err)
				}
				output = buf.String()
			}

			if len(output) > 4000 {
				output = output[:4000] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"format": args.Format, "length": len(output)},
			}, nil
		},
	}
}
