package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// XMLQueryTool creates an XML query tool.
//
// Parameters:
//
//	file  (string, required) — XML file path
//	query (string, optional) — dot-separated path with tag names (e.g., "root.items.0.name")
func XMLQueryTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {"type": "string", "description": "XML file path"},
			"query": {"type": "string", "description": "Dot-separated path with tag names (e.g., root.items.0.name). Empty = return entire doc."}
		},
		"required": ["file"]
	}`)

	return core.Tool{
		Name:        "xml_query",
		Description: "Parse and query XML files using dot-notation tag paths with array indexing.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File  string `json:"file"`
				Query string `json:"query"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}

			data, err := os.ReadFile(args.File)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read file: %w", err)
			}

			root := &xmlNode{}
			if err := xml.Unmarshal(data, root); err != nil {
				return core.ToolResult{}, fmt.Errorf("parse xml: %w", err)
			}

			// Convert to generic map with root element name for path navigation
			obj := map[string]any{root.XMLName.Local: nodeToMap(root)}
			var result any = obj
			if args.Query != "" {
				result = navigateJSON(obj, args.Query)
			}

			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: string(resultJSON)}},
				Details: map[string]any{"file": args.File, "query": args.Query, "success": true},
			}, nil
		},
	}
}

type xmlNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content []byte     `xml:",innerxml"`
	Nodes   []xmlNode  `xml:",any"`
}

func nodeToMap(n *xmlNode) map[string]any {
	if n == nil {
		return nil
	}
	m := make(map[string]any)

	// Add text content if present
	text := strings.TrimSpace(string(n.Content))
	if text != "" && !strings.HasPrefix(text, "<") {
		m["_text"] = text
	}

	// Add attributes
	for _, attr := range n.Attrs {
		m["@"+attr.Name.Local] = attr.Value
	}

	// Add child nodes
	for _, child := range n.Nodes {
		name := child.XMLName.Local
		childMap := nodeToMap(&child)
		if existing, ok := m[name]; ok {
			// Convert to array if already exists
			switch v := existing.(type) {
			case []any:
				m[name] = append(v, childMap)
			default:
				m[name] = []any{existing, childMap}
			}
		} else {
			m[name] = childMap
		}
	}

	return m
}
