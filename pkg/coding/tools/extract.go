package tools

import "encoding/json"

// FileOp describes a file operation extracted from a tool call.
type FileOp struct {
	Path   string // file path (relative)
	Action string // "read", "write", "edit", "glob", "grep"
}

// ExtractFileOps extracts file operations from tool parameters.
// Returns nil if the tool name is not a file-touching tool.
func ExtractFileOps(toolName string, params any) []FileOp {
	raw, _ := json.Marshal(params)
	switch toolName {
	case "read":
		var args struct{ FilePath string `json:"file_path"` }
		json.Unmarshal(raw, &args)
		if args.FilePath != "" {
			return []FileOp{{Path: args.FilePath, Action: "read"}}
		}
	case "write":
		var args struct{ FilePath string `json:"file_path"` }
		json.Unmarshal(raw, &args)
		if args.FilePath != "" {
			return []FileOp{{Path: args.FilePath, Action: "write"}}
		}
	case "edit":
		var args struct{ FilePath string `json:"file_path"` }
		json.Unmarshal(raw, &args)
		if args.FilePath != "" {
			return []FileOp{{Path: args.FilePath, Action: "edit"}}
		}
	case "glob":
		var args struct{ Pattern string `json:"pattern"` }
		json.Unmarshal(raw, &args)
		if args.Pattern != "" {
			return []FileOp{{Path: args.Pattern, Action: "glob"}}
		}
	case "grep":
		var args struct{ Path string `json:"path"` }
		json.Unmarshal(raw, &args)
		if args.Path != "" {
			return []FileOp{{Path: args.Path, Action: "grep"}}
		}
	}
	return nil
}

// FormatFileOp formats a tool call as a human-readable file operation.
func FormatFileOp(toolName string, params any) string {
	ops := ExtractFileOps(toolName, params)
	if len(ops) == 0 {
		return ""
	}
	return ops[0].Action + ": " + ops[0].Path
}
