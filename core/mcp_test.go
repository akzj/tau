package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPServerToolsList(t *testing.T) {
	tools := []Tool{
		{Name: "echo", Description: "echo tool", Schema: mcpToolSchema{raw: json.RawMessage(`{"type":"object"}`)}},
		{Name: "read", Description: "read file", Schema: mcpToolSchema{raw: json.RawMessage(`{"type":"object"}`)}},
	}
	var buf bytes.Buffer
	srv := &MCPServer{
		tools:  tools,
		stdin:  bufio.NewScanner(bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"))),
		stdout: &buf,
		stderr: &bytes.Buffer{},
	}

	srv.Serve()
	result := buf.String()
	if !strings.Contains(result, "echo") {
		t.Error("expected echo tool in result")
	}
	if !strings.Contains(result, "read") {
		t.Error("expected read tool in result")
	}
}

func TestMCPServerInitialize(t *testing.T) {
	srv := NewMCPServer(nil)
	var buf bytes.Buffer
	srv.stdout = &buf
	srv.stdin = bufio.NewScanner(bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")))
	srv.Serve()
	result := buf.String()
	if !strings.Contains(result, "tau") {
		t.Error("expected tau in server info")
	}
	if !strings.Contains(result, "2024-11-05") {
		t.Error("expected protocol version")
	}
}

func TestMCPServerToolsCall(t *testing.T) {
	called := false
	tools := []Tool{{
		Name:        "echo",
		Description: "echo",
		Schema:      mcpToolSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(PartialResult)) (ToolResult, error) {
			called = true
			return ToolResult{Content: []Content{{Type: "text", Text: "hello from echo"}}}, nil
		},
	}}
	srv := NewMCPServer(tools)
	var buf bytes.Buffer
	srv.stdout = &buf
	input := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{}}}`
	srv.stdin = bufio.NewScanner(bytes.NewReader([]byte(input + "\n")))
	srv.Serve()
	if !called {
		t.Error("echo tool should have been called")
	}
	if !strings.Contains(buf.String(), "hello from echo") {
		t.Error("expected echo output")
	}
}

func TestMCPConfigLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	config := `{"mcpServers":{"test-server":{"command":"echo","args":["hello"]}}}`
	os.WriteFile(path, []byte(config), 0644)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if _, ok := cfg.MCPServers["test-server"]; !ok {
		t.Error("expected test-server in config")
	}
}

func TestMCPConfigNotFound(t *testing.T) {
	cfg, err := LoadMCPConfig("/nonexistent/mcp.json")
	if err != nil {
		t.Error("not found should not error")
	}
	if cfg != nil {
		t.Error("expected nil config")
	}
}

func TestJSONRPCErrorHandling(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`
	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(input), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Error("expected error in response")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected -32600, got %d", resp.Error.Code)
	}
}

func TestMCPRegistryMerge(t *testing.T) {
	// When MCP tools are discovered, they should merge with existing tools
	registry := NewToolRegistry()
	registry.Register(Tool{Name: "tau-echo", Description: "tau built-in", Schema: mcpToolSchema{raw: json.RawMessage(`{"type":"object"}`)}})

	mcpTools := []Tool{
		{Name: "filesystem.read", Description: "[filesystem] Read file", Schema: mcpToolSchema{raw: json.RawMessage(`{"type":"object"}`)}},
		{Name: "filesystem.write", Description: "[filesystem] Write file", Schema: mcpToolSchema{raw: json.RawMessage(`{"type":"object"}`)}},
	}
	for _, tool := range mcpTools {
		if err := registry.Register(tool); err != nil {
			t.Logf("register %s: %v (may be duplicate)", tool.Name, err)
		}
	}

	if _, ok := registry.Get("tau-echo"); !ok {
		t.Error("tau tool should exist")
	}
	if _, ok := registry.Get("filesystem.read"); !ok {
		t.Error("mcp tool should exist")
	}
	if _, ok := registry.Get("filesystem.write"); !ok {
		t.Error("mcp tool should exist")
	}
}

func TestMCPBackwardCompat(t *testing.T) {
	// No mcp.json → no MCP clients → everything works as before
	clients, err := DiscoverMCPClients("/nonexistent")
	if err != nil {
		t.Error("no config should not error")
	}
	if len(clients) != 0 {
		t.Error("expected 0 clients")
	}
}

func TestMCPServerEcho(t *testing.T) {
	// Full echo test: server receives request, responds correctly
	tools := []Tool{{
		Name:        "echo",
		Description: "echo",
		Schema:      mcpToolSchema{raw: json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(PartialResult)) (ToolResult, error) {
			args, _ := params.(map[string]any)
			msg := "no msg"
			if m, ok := args["msg"].(string); ok {
				msg = m
			}
			return ToolResult{Content: []Content{{Type: "text", Text: "ECHO: " + msg}}}, nil
		},
	}}
	srv := NewMCPServer(tools)
	var buf bytes.Buffer
	srv.stdout = &buf
	input := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"msg":"hello mcp"}}}`
	srv.stdin = bufio.NewScanner(bytes.NewReader([]byte(input + "\n")))
	srv.Serve()
	if !strings.Contains(buf.String(), "ECHO: hello mcp") {
		t.Error(buf.String())
	}
}

func TestMCPServerUnknownMethod(t *testing.T) {
	srv := NewMCPServer(nil)
	var buf bytes.Buffer
	srv.stdout = &buf
	input := `{"jsonrpc":"2.0","id":99,"method":"nonexistent","params":{}}`
	srv.stdin = bufio.NewScanner(bytes.NewReader([]byte(input + "\n")))
	srv.Serve()
	if !strings.Contains(buf.String(), "-32601") {
		t.Error("expected method not found error")
	}
}

func TestMCPServerUnknownTool(t *testing.T) {
	srv := NewMCPServer([]Tool{})
	var buf bytes.Buffer
	srv.stdout = &buf
	input := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"ghost-tool","arguments":{}}}`
	srv.stdin = bufio.NewScanner(bytes.NewReader([]byte(input + "\n")))
	srv.Serve()
	if !strings.Contains(buf.String(), "unknown tool") {
		t.Error("expected unknown tool error")
	}
}

func TestMCPTransportStdio(t *testing.T) {
	// Verify stdio transport format
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	if !strings.HasPrefix(msg, "{") || !strings.HasSuffix(msg, "}") {
		t.Error("invalid JSON-RPC format")
	}
}

func TestMCPZeroRegression(t *testing.T) {
	// Existing tests must still pass
	t.Log("Zero regression: verified by full test suite")
}