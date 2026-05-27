//go:build !no_cohere

package cohere

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// Provider implements core.Provider for Cohere (v2/chat API).
// Cohere v2 is NOT OpenAI-compatible — it uses its own request/response shape.
type Provider struct {
	baseURL   string
	apiKey    string
	client    *http.Client
	maxTokens int
}

// New creates a provider with an explicit API key (used in tests).
func New(apiKey string) *Provider {
	return &Provider{
		baseURL:   "https://api.cohere.com/v2",
		apiKey:    apiKey,
		client:    &http.Client{},
		maxTokens: 4096,
	}
}

// NewProvider creates a Cohere provider from environment variables.
//
// Env vars:
//
//	COHERE_API_KEY (required) — API key
//	COHERE_BASE_URL (default: "https://api.cohere.com/v2")
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv("COHERE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("COHERE_API_KEY not set")
	}
	baseURL := os.Getenv("COHERE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.cohere.com/v2"
	}
	return &Provider{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		apiKey:    apiKey,
		client:    &http.Client{},
		maxTokens: 4096,
	}, nil
}

// --- core.Provider implementation --------------------------------------------

// Complete sends a non-streaming chat completion request.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	body := p.buildRequest(core.StreamRequest{
		Model:        req.Model,
		Messages:     req.Messages,
		SystemPrompt: req.SystemPrompt,
		Options:      req.Options,
	}, false)

	httpReq, err := p.newRequest(ctx, body)
	if err != nil {
		return core.CompleteResponse{}, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return core.CompleteResponse{}, fmt.Errorf("cohere.Complete: http request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return core.CompleteResponse{}, fmt.Errorf("cohere.Complete: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return core.CompleteResponse{}, p.classifyError("cohere.Complete", resp.StatusCode, bodyBytes)
	}

	var result struct {
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Usage struct {
			BilledUnits struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"billed_units"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return core.CompleteResponse{}, fmt.Errorf("cohere.Complete: decode response: %w", err)
	}

	// Accumulate text from content blocks.
	var content strings.Builder
	for _, block := range result.Message.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	return core.CompleteResponse{
		Content: content.String(),
		Usage: core.Usage{
			PromptTokens:     result.Usage.BilledUnits.InputTokens,
			CompletionTokens: result.Usage.BilledUnits.OutputTokens,
			TotalTokens:      result.Usage.BilledUnits.InputTokens + result.Usage.BilledUnits.OutputTokens,
		},
	}, nil
}

// Stream sends a streaming chat completion request.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	body := p.buildRequest(req, true)

	// OnPayload transform hook.
	if req.OnPayload != nil {
		transformed, err := req.OnPayload(body)
		if err != nil {
			return nil, fmt.Errorf("cohere.Stream: OnPayload: %w", err)
		}
		var ok bool
		body, ok = transformed.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cohere.Stream: OnPayload: expected map[string]any, got %T", transformed)
		}
	}

	httpReq, err := p.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cohere.Stream: http request: %w", err)
	}

	// OnResponse hook.
	if req.OnResponse != nil {
		req.OnResponse(resp.StatusCode, resp.Header)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, p.classifyError("cohere.Stream", resp.StatusCode, bodyBytes)
	}

	ch := make(chan core.ProviderEvent, 64)
	go p.parseSSE(ctx, resp.Body, ch)
	return ch, nil
}

// CountTokens estimates the number of tokens in a text.
func (p *Provider) CountTokens(text string) int {
	return len(text) / 4
}

// Compat returns wire-specific compatibility flags.
func (p *Provider) Compat() core.WireCompat {
	return core.OpenAICompletionsCompat{
		TemperatureField: true,
		SupportsStop:     true,
		MaxTokensField:   true,
	}
}

// --- Request building --------------------------------------------------------

// buildRequest constructs the Cohere v2 Chat API request body.
func (p *Provider) buildRequest(req core.StreamRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":    req.Model.Name,
		"messages": convertMessages(req.Messages, req.SystemPrompt),
		"stream":   stream,
	}

	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	} else {
		body["max_tokens"] = p.maxTokens
	}

	if req.Options.Temperature > 0 {
		body["temperature"] = req.Options.Temperature
	}

	if len(req.Tools) > 0 {
		body["tools"] = convertTools(req.Tools, req.TransformToolName)
	}

	return body
}

// newRequest creates an HTTP request with Bearer token auth.
func (p *Provider) newRequest(ctx context.Context, body map[string]any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("cohere.Stream: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("cohere.Stream: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	return httpReq, nil
}

// --- Message conversion ------------------------------------------------------

// convertMessages converts core.Message slice to Cohere v2 Chat format.
// Cohere v2 supports standard role/content messages (OpenAI-style).
//
// Supports:
//   - Plain text messages
//   - System prompt as first message
//   - Assistant tool_calls messages
//   - Tool result messages (tool_call_id)
func convertMessages(msgs []core.Message, systemPrompt string) []map[string]any {
	var out []map[string]any

	// System prompt as first message.
	if systemPrompt != "" {
		out = append(out, map[string]any{
			"role":    "system",
			"content": systemPrompt,
		})
	}

	for _, msg := range msgs {
		// System messages handled via systemPrompt.
		if msg.Role == core.RoleSystem {
			continue
		}

		m := map[string]any{"role": string(msg.Role)}

		switch {
		case msg.Role == core.RoleAssistant && len(msg.ToolCalls) > 0:
			// Assistant message with tool_calls.
			var tcs []map[string]any
			for _, tc := range msg.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id":   tc.CallID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.ToolName,
						"arguments": tc.Args,
					},
				})
			}
			m["tool_calls"] = tcs
			if msg.Content != "" {
				m["content"] = msg.Content
			}

		case msg.Role == core.RoleTool:
			// Tool result → role:tool with tool_call_id.
			m["tool_call_id"] = msg.ToolCallID
			m["content"] = msg.Content

		default:
			m["content"] = msg.Content
		}

		out = append(out, m)
	}

	return out
}

// --- Tool conversion ---------------------------------------------------------

// convertTools converts core.ToolSpec slice to Cohere format.
//
// Cohere uses "parameter_definitions" (flat map) instead of
// OpenAI-style "parameters" (JSON Schema object).
func convertTools(tools []core.ToolSpec, transform func(string) string) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		name := t.Name
		if transform != nil {
			name = transform(name)
		}
		tool := map[string]any{
			"name":        name,
			"description": t.Description,
		}
		// Extract properties from JSON Schema into parameter_definitions.
		if schema, ok := t.Schema.(map[string]any); ok {
			if props, ok := schema["properties"]; ok {
				tool["parameter_definitions"] = props
			} else {
				tool["parameter_definitions"] = map[string]any{}
			}
		}
		out = append(out, tool)
	}
	return out
}

// --- SSE parsing -------------------------------------------------------------

// cohereSSEChunk is the JSON payload of a Cohere SSE data line.
type cohereSSEChunk struct {
	Type  string `json:"type"` // content-start, content-delta, content-end, tool-call-start, tool-call-delta, tool-call-end
	Index *int   `json:"index"`
	Delta *struct {
		Message *struct {
			Content *struct {
				Text string `json:"text"`
			} `json:"content"`
			ToolCalls *struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function *struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

// parseSSE reads Cohere SSE streaming responses and emits ProviderEvents.
func (p *Provider) parseSSE(ctx context.Context, body io.ReadCloser, ch chan<- core.ProviderEvent) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)
	var (
		msgID             string
		currentToolCallID string
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var chunk cohereSSEChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("cohere.Stream: parse chunk: %w", err)}
			continue
		}

		switch chunk.Type {
		case "content-start":
			// Content block starts — emit message start if not already.
			if msgID == "" {
				msgID = "msg-cohere"
				ch <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
			}

		case "content-delta":
			// Text delta.
			if chunk.Delta != nil && chunk.Delta.Message != nil && chunk.Delta.Message.Content != nil {
				ch <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    msgID,
					ContentDelta: chunk.Delta.Message.Content.Text,
				}
			}

		case "content-end":
			// Content block ends. May carry finish_reason.
			if msgID != "" && chunk.FinishReason != "" {
				ch <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
			}

		case "tool-call-start":
			// Tool call begins — emit ProvMessageStart if not already.
			if msgID == "" {
				msgID = "msg-cohere"
				ch <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
			}

			if chunk.Delta != nil && chunk.Delta.Message != nil && chunk.Delta.Message.ToolCalls != nil {
				tc := chunk.Delta.Message.ToolCalls
				currentToolCallID = tc.ID
				toolName := ""
				if tc.Function != nil {
					toolName = tc.Function.Name
				}
				ch <- core.ProviderEvent{
					Type:       core.ProvToolCallStart,
					MessageID:  msgID,
					ToolCallID: currentToolCallID,
					ToolName:   toolName,
				}
			}

		case "tool-call-delta":
			// Tool argument delta.
			if chunk.Delta != nil && chunk.Delta.Message != nil && chunk.Delta.Message.ToolCalls != nil {
				tc := chunk.Delta.Message.ToolCalls
				if tc.Function != nil && tc.Function.Arguments != "" {
					ch <- core.ProviderEvent{
						Type:          core.ProvToolCallDelta,
						MessageID:     msgID,
						ToolCallID:    currentToolCallID,
						ToolArgsDelta: tc.Function.Arguments,
					}
				}
			}

		case "tool-call-end":
			// Tool call ends.
			if currentToolCallID != "" {
				ch <- core.ProviderEvent{
					Type:       core.ProvToolCallEnd,
					MessageID:  msgID,
					ToolCallID: currentToolCallID,
				}
				currentToolCallID = ""
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("cohere.Stream: SSE scan: %w", err)}
	}

	// Emit final message end if stream ended without content-end finish_reason.
	if msgID != "" {
		ch <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
	}
}

// --- Error classification ----------------------------------------------------

// classifyError returns a structured core.Error based on HTTP status code.
//
// Classification:
//   - 429 → Transient (rate limited)
//   - 400 → UsageError (bad request)
//   - 401/403 → Permanent (auth)
//   - 5xx → Transient (server error)
//   - default → Permanent
//
// Cohere error body: {"message": "error description"}
func (p *Provider) classifyError(op string, statusCode int, body []byte) error {
	var errResp struct {
		Message string `json:"message"`
	}
	json.Unmarshal(body, &errResp)
	msg := errResp.Message
	if msg == "" {
		msg = string(body)
	}

	switch {
	case statusCode == 429:
		return core.Transient(op, fmt.Errorf("rate limited (429): %s", msg))
	case statusCode == 400:
		return core.UsageError(op, fmt.Errorf("bad request (400): %s", msg))
	case statusCode == 401 || statusCode == 403:
		return core.Permanent(op, fmt.Errorf("auth error (%d): %s", statusCode, msg))
	case statusCode >= 500:
		return core.Transient(op, fmt.Errorf("server error (%d): %s", statusCode, msg))
	default:
		return core.Permanent(op, fmt.Errorf("API error %d: %s", statusCode, msg))
	}
}
