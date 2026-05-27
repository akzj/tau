package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// mcpToolSchema adapts a raw JSON schema to ToolSchema for MCP-discovered tools.
type mcpToolSchema struct {
	raw json.RawMessage
}

func (s mcpToolSchema) Marshal() (json.RawMessage, error)      { return s.raw, nil }
func (s mcpToolSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// MCPClient connects to an MCP server via stdio or SSE transport.
type MCPClient struct {
	mu          sync.Mutex
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Scanner
	reqID       int
	tools       []Tool
	serverName  string
	transport   string // "stdio" or "sse"
}

// MCPConfig holds MCP server configuration (Claude Desktop format).
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig configures a single MCP server.
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"` // for SSE transport
}

// NewMCPClient creates and connects to an MCP server.
func NewMCPClient(name string, cfg MCPServerConfig) (*MCPClient, error) {
	client := &MCPClient{reqID: 1, serverName: name}

	if cfg.URL != "" {
		return nil, fmt.Errorf("SSE transport not yet implemented — use stdio")
	}

	// Stdio transport
	client.transport = "stdio"
	client.cmd = exec.Command(cfg.Command, cfg.Args...)
	for k, v := range cfg.Env {
		client.cmd.Env = append(client.cmd.Env, k+"="+v)
	}

	var err error
	client.stdin, err = client.cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdin: %w", err)
	}
	stdoutPipe, err := client.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdout: %w", err)
	}
	client.stdout = bufio.NewScanner(stdoutPipe)

	if err := client.cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp start: %w", err)
	}

	// Initialize
	if _, err := client.call(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "tau", "version": "0.1.0"},
	}); err != nil {
		client.Close()
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}

	// Discover tools
	tools, err := client.ListTools(context.Background())
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}
	client.tools = tools

	return client, nil
}

// ListTools returns tools available from this MCP server.
func (c *MCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse tools/list: %w", err)
	}

	var tools []Tool
	for _, t := range resp.Tools {
		tools = append(tools, Tool{
			Name:        c.serverName + "." + t.Name, // prefix with server name
			Description: "[" + c.serverName + "] " + t.Description,
			Schema:      mcpToolSchema{raw: t.InputSchema},
		})
	}
	return tools, nil
}

// CallTool executes a tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	// Strip server prefix if present
	if strings.HasPrefix(toolName, c.serverName+".") {
		toolName = strings.TrimPrefix(toolName, c.serverName+".")
	}
	result, err := c.call(ctx, "tools/call", map[string]any{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(result, &resp)

	var text string
	for _, c := range resp.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	return text, nil
}

// call sends a JSON-RPC request and returns the result.
func (c *MCPClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.reqID
	c.reqID++
	c.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	data, _ := json.Marshal(req)
	fmt.Fprintf(c.stdin, "%s\n", data)

	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("mcp: unexpected EOF")
	}

	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("mcp parse: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

// Tools returns discovered tools.
func (c *MCPClient) Tools() []Tool { return c.tools }

// ServerName returns the configured server name.
func (c *MCPClient) ServerName() string { return c.serverName }

// Close shuts down the connection.
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}