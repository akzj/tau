package toolspec

import (
	"encoding/json"
	"fmt"

	"github.com/akzj/tau/core"
)

// EchoToolSchema implements core.ToolSchema for the echo tool.
// It uses the codegen-generated EchoSchemaJson type for validation.
type EchoToolSchema struct{}

// Marshal returns the LLM-facing JSON Schema for the echo tool.
func (EchoToolSchema) Marshal() (json.RawMessage, error) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"msg": map[string]any{
				"type":        "string",
				"description": "The message to echo back",
			},
		},
		"required":             []string{"msg"},
		"additionalProperties": false,
	}
	return json.Marshal(schema)
}

// Validate decodes and validates raw JSON against the echo schema.
func (EchoToolSchema) Validate(raw json.RawMessage) (any, error) {
	var v EchoSchemaJson
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("echo: invalid args: %w", err)
	}
	if v.Msg == "" {
		return nil, fmt.Errorf("echo: missing required field: msg")
	}
	return v, nil
}

// Compile-time check: EchoToolSchema implements core.ToolSchema.
var _ core.ToolSchema = EchoToolSchema{}
