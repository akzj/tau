package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/akzj/tau/core"
)

// AskUserTool creates a tool that pauses the loop and asks the user a question.
func AskUserTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"question": {"type": "string", "description": "The question to ask the user"},
			"options": {"type": "array", "items": {"type": "string"}, "description": "Optional: allowed response options"}
		},
		"required": ["question"]
	}`)

	return core.Tool{
		Name:        "ask_user",
		Description: "Pause and ask the user a question. Loop waits for user response before continuing.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Question string   `json:"question"`
				Options  []string `json:"options"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Question == "" {
				return core.ToolResult{}, fmt.Errorf("question required")
			}

			output := "❓ " + args.Question
			if len(args.Options) > 0 {
				output += "\nOptions: " + strings.Join(args.Options, ", ")
			}
			output += "\n\n[Awaiting user response...]"

			return core.ToolResult{
				Content:   []core.Content{{Type: "text", Text: output}},
				Details:   map[string]any{"question": args.Question, "options": args.Options},
				Terminate: true, // Signal loop to pause
			}, nil
		},
	}
}
