package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// MCPServer exposes tau tools as an MCP server over stdio.
type MCPServer struct {
	tools  []Tool
	stdin  *bufio.Scanner
	stdout io.Writer
	stderr io.Writer
}

// NewMCPServer creates an MCP server from a tool registry.
func NewMCPServer(tools []Tool) *MCPServer {
	return &MCPServer{
		tools:  tools,
		stdin:  bufio.NewScanner(os.Stdin),
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// Serve starts the MCP JSON-RPC server loop on stdio.
func (s *MCPServer) Serve() error {
	for s.stdin.Scan() {
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(s.stdin.Bytes(), &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			s.respond(req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]string{"name": "tau", "version": "0.1.0"},
			})
		case "tools/list":
			s.handleToolsList(req.ID)
		case "tools/call":
			s.handleToolsCall(req.ID, req.Params)
		default:
			s.respondError(req.ID, -32601, fmt.Sprintf("unknown method: %s", req.Method))
		}
	}
	return s.stdin.Err()
}

func (s *MCPServer) handleToolsList(id int) {
	var tools []map[string]any
	for _, t := range s.tools {
		var schema any
		if t.Schema != nil {
			raw, _ := t.Schema.Marshal()
			json.Unmarshal(raw, &schema)
		}
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
		})
	}
	s.respond(id, map[string]any{"tools": tools})
}

func (s *MCPServer) handleToolsCall(id int, params json.RawMessage) {
	var req struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		s.respondError(id, -32602, "invalid params")
		return
	}

	// Find matching tool
	for _, t := range s.tools {
		if t.Name == req.Name {
			// Execute via existing tool infrastructure
			result, err := t.Execute(context.Background(), fmt.Sprintf("mcp-%d", id), req.Arguments, nil)
			if err != nil {
				s.respond(id, map[string]any{
					"content": []map[string]any{{"type": "text", "text": err.Error()}},
					"isError": true,
				})
				return
			}
			var text string
			for _, c := range result.Content {
				if c.Type == "text" {
					text += c.Text
				}
			}
			s.respond(id, map[string]any{
				"content": []map[string]any{{"type": "text", "text": text}},
			})
			return
		}
	}
	s.respondError(id, -32602, fmt.Sprintf("unknown tool: %s", req.Name))
}

func (s *MCPServer) respond(id int, result any) {
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	fmt.Fprintf(s.stdout, "%s\n", data)
}

func (s *MCPServer) respondError(id int, code int, message string) {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id,
		"error": map[string]any{"code": code, "message": message},
	})
	fmt.Fprintf(s.stdout, "%s\n", data)
}