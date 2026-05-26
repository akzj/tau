package tools

import (
	"encoding/json"
	"fmt"

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

// Validate checks JSON validity, then unmarshals into a generic value.
func (s Schema) Validate(raw json.RawMessage) (any, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("schema: invalid JSON")
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Compile-time check: Schema implements core.ToolSchema.
var _ core.ToolSchema = Schema{}
