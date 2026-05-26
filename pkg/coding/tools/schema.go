package tools

import (
	"encoding/json"

	"github.com/akzj/tau/core"
)

// Schema wraps a raw JSON Schema and implements core.ToolSchema.
// It passes through params as-is in Validate (the Execute function handles parsing).
type Schema struct {
	Raw json.RawMessage
}

// Marshal returns the LLM-facing JSON Schema.
func (s Schema) Marshal() (json.RawMessage, error) {
	return s.Raw, nil
}

// Validate passes through raw JSON params as-is.
func (s Schema) Validate(raw json.RawMessage) (any, error) {
	return raw, nil
}

// Compile-time check: Schema implements core.ToolSchema.
var _ core.ToolSchema = Schema{}
