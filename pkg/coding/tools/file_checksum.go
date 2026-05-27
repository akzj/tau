package tools

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/akzj/tau/core"
)

// FileChecksumTool creates a file checksum tool.
//
// Parameters:
//
//	path      (string, required) — file path
//	algorithm (string, optional) — sha256 | md5 | sha512 (default: sha256)
func FileChecksumTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path to checksum"},
			"algorithm": {"type": "string", "description": "Hash algorithm: sha256, md5, sha512 (default: sha256)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "file_checksum",
		Description: "Compute SHA256, MD5, or SHA512 checksum of a file.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path      string `json:"path"`
				Algorithm string `json:"algorithm"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.Algorithm == "" {
				args.Algorithm = "sha256"
			}

			path := args.Path
			if !filepath.IsAbs(path) {
				path = filepath.Join(WorkspaceRoot, path)
			}

			f, err := os.Open(path)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("open file: %w", err)
			}
			defer f.Close()

			var h hash.Hash
			switch args.Algorithm {
			case "sha256":
				h = sha256.New()
			case "md5":
				h = md5.New()
			case "sha512":
				h = sha512.New()
			default:
				return core.ToolResult{}, fmt.Errorf("unsupported algorithm: %s (use sha256/md5/sha512)", args.Algorithm)
			}

			if _, err := io.Copy(h, f); err != nil {
				return core.ToolResult{}, fmt.Errorf("hash: %w", err)
			}

			checksum := hex.EncodeToString(h.Sum(nil))
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("%s: %s", args.Algorithm, checksum)}},
				Details: map[string]any{"path": args.Path, "algorithm": args.Algorithm, "checksum": checksum, "success": true},
			}, nil
		},
	}
}
