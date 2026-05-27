//go:build !no_anthropic

package anthropic_messages

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
	"time"

	"github.com/akzj/tau/core"
)

const (
	anthropicMessagesURL = "https://api.anthropic.com/v1/messages"
	anthropicVersion     = "2023-06-01"
)

// --- Provider type -----------------------------------------------------------

// AnthropicMessagesProvider implements core.Provider for Anthropic Messages API.
type AnthropicMessagesProvider struct {
	baseURL   string
	apiKey    string
	client    *http.Client
	maxTokens int
	// streamFn allows test injection of the HTTP round-trip.
	streamFn func(ctx context.Context, req *http.Request) (*http.Response, error)
}

// New creates a provider with an explicit API key (used in tests).
func New(apiKey string) *AnthropicMessagesProvider {
	return &AnthropicMessagesProvider{
		baseURL:   anthropicMessagesURL,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 120 * time.Second},
		maxTokens: 4096,
	}
}

// NewAnthropicMessagesProvider creates a provider from environment variables.
func NewAnthropicMessagesProvider() (*AnthropicMessagesProvider, error) {
	apiKey := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_AUTH_TOKEN not set")
	}
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://athenai.mihoyo.com"
	}
	return &AnthropicMessagesProvider{
		baseURL:   strings.TrimSuffix(baseURL, "/") + "/v1",
		apiKey:    apiKey,
		client:    &http.Client{},
		maxTokens: 4096,
	}, nil
}

// newTestProvider creates a provider pointing at an httptest server.
// Used by tests in this package.
func newTestProvider(baseURL string, client *http.Client) *AnthropicMessagesProvider {
	return &AnthropicMessagesProvider{
		baseURL:   baseURL,
		apiKey:    "test-key",
		client:    client,
		maxTokens: 100,
	}
}

// --- core.Provider implementation --------------------------------------------

// Complete sends a non-streaming Messages API request.
func (p *AnthropicMessagesProvider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
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

	resp, err := p.doRequest(httpReq)
	if err != nil {
		return core.CompleteResponse{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return core.CompleteResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return core.CompleteResponse{}, p.classifyError("anthropic.Complete", resp.StatusCode, bodyBytes)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return core.CompleteResponse{}, fmt.Errorf("decode response: %w", err)
	}

	var content string
	for _, c := range result.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return core.CompleteResponse{
		Content: content,
		Usage: core.Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
	}, nil
}

// Stream sends a streaming Messages API request.
func (p *AnthropicMessagesProvider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	body := p.buildRequest(req, true)

	// OnPayload transform hook (backward compat).
	if req.OnPayload != nil {
		transformed, err := req.OnPayload(body)
		if err != nil {
			return nil, fmt.Errorf("OnPayload: %w", err)
		}
		var ok bool
		body, ok = transformed.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("OnPayload: expected map[string]any, got %T", transformed)
		}
	}

	httpReq, err := p.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	// OnResponse hook (backward compat).
	if req.OnResponse != nil {
		req.OnResponse(resp.StatusCode, resp.Header)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, p.classifyError("anthropic.Stream", resp.StatusCode, bodyBytes)
	}

	ch := make(chan core.ProviderEvent, 64)
	go p.parseSSE(ctx, resp.Body, ch)
	return ch, nil
}

// Compat returns wire-specific compatibility flags.
func (p *AnthropicMessagesProvider) Compat() core.WireCompat {
	return core.AnthropicMessagesCompat{
		ThinkingFormat:        false,
		SupportsCacheControl:  false,
		MaxTokensField:        true,
		SupportsStopSequences: true,
		SupportsTopK:          false,
		TemperatureField:      true,
		SupportsToolChoice:    true,
	}
}

// --- request building --------------------------------------------------------

// buildRequest constructs the Anthropic Messages API request body.
//
// Requirements:
//  1. Full POST /v1/messages body: model, max_tokens, messages, system, tools, tool_choice, stop_sequences, stream.
//  3. Vision: image content blocks with base64 source.
//  4. Tool Use: tools[] → input_schema format with tool_choice: {type: "auto"}.
func (p *AnthropicMessagesProvider) buildRequest(req core.StreamRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":      req.Model.Name,
		"max_tokens": p.maxTokens,
		"stream":     stream,
	}

	// Override max_tokens from options if provided.
	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	}

	// System prompt as top-level field.
	if req.SystemPrompt != "" {
		body["system"] = req.SystemPrompt
	}

	// Messages in Anthropic content-block format (including vision).
	body["messages"] = convertMessages(req.Messages)

	// Tools in Anthropic input_schema format.
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, t := range req.Tools {
			name := t.Name
			if req.TransformToolName != nil {
				name = req.TransformToolName(name)
			}
			tool := map[string]any{
				"name":        name,
				"description": t.Description,
			}
			// Parse Schema to unwrap JSON if needed.
			if t.Schema != nil {
				tool["input_schema"] = t.Schema
			}
			tools = append(tools, tool)
		}
		body["tools"] = tools
		body["tool_choice"] = map[string]string{"type": "auto"}
	}

	// Stop sequences.
	if len(req.Options.Stop) > 0 {
		body["stop_sequences"] = req.Options.Stop
	}

	return body
}

// newRequest creates an HTTP request with required Anthropic headers.
func (p *AnthropicMessagesProvider) newRequest(ctx context.Context, body map[string]any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	return httpReq, nil
}

// doRequest performs the HTTP round-trip, using streamFn for test injection.
func (p *AnthropicMessagesProvider) doRequest(req *http.Request) (*http.Response, error) {
	if p.streamFn != nil {
		return p.streamFn(req.Context(), req)
	}
	return p.client.Do(req)
}

// --- message building (requirement 3: vision, requirement 4: tool use) --------

// ContentBlock represents a single content block in an Anthropic message.
type ContentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *ImageSource `json:"source,omitempty"`
}

// ImageSource is the base64 image source for vision content blocks.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// convertMessages converts core.Message slice to Anthropic Messages format.
//
// Supports:
//   - Plain text messages
//   - Vision content blocks (JSON-encoded in Content field)
//   - Assistant tool_use messages
//   - Tool result messages
func convertMessages(messages []core.Message) []map[string]any {
	var msgs []map[string]any
	for _, m := range messages {
		// System messages are handled via top-level "system" field.
		if m.Role == core.RoleSystem {
			continue
		}

		msg := map[string]any{"role": string(m.Role)}

		switch {
		case m.Role == core.RoleAssistant && len(m.ToolCalls) > 0:
			// Assistant message with tool_use blocks.
			var content []map[string]any
			if m.Content != "" {
				content = append(content, map[string]any{
					"type": "text",
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				var args map[string]any
				if tc.Args != "" {
					json.Unmarshal([]byte(tc.Args), &args)
				}
				if args == nil {
					args = map[string]any{}
				}
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    tc.CallID,
					"name":  tc.ToolName,
					"input": args,
				})
			}
			msg["content"] = content

		case m.Role == core.RoleTool:
			// Tool result → user message with tool_result content block.
			msg = map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":         "tool_result",
					"tool_use_id":  m.ToolCallID,
					"content":      m.Content,
				}},
			}

		default:
			// Check for structured content blocks (vision support via JSON encoding).
			if blocks := parseContentBlocks(m.Content); blocks != nil {
				msg["content"] = blocks
			} else {
				msg["content"] = m.Content
			}
		}

		msgs = append(msgs, msg)
	}
	return msgs
}

// parseContentBlocks attempts to parse s as a JSON array of ContentBlock.
// Returns nil if parsing fails — caller should treat s as plain text.
func parseContentBlocks(s string) []map[string]any {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return nil
	}
	var blocks []ContentBlock
	if err := json.Unmarshal([]byte(s), &blocks); err != nil {
		return nil
	}
	// Convert to map[string]any for message building.
	var out []map[string]any
	for _, b := range blocks {
		m := map[string]any{"type": b.Type}
		if b.Text != "" {
			m["text"] = b.Text
		}
		if b.Source != nil {
			m["source"] = map[string]any{
				"type":       b.Source.Type,
				"media_type": b.Source.MediaType,
				"data":       b.Source.Data,
			}
		}
		out = append(out, m)
	}
	return out
}

// --- SSE parsing (requirement 2: streaming) ----------------------------------

// anthropicSSEData is the JSON payload of an SSE event.
type anthropicSSEData struct {
	Type    string `json:"type"`
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Role  string `json:"role"`
	} `json:"message"`
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// parseSSE reads Anthropic SSE streaming responses and emits ProviderEvents.
func (p *AnthropicMessagesProvider) parseSSE(ctx context.Context, body io.ReadCloser, ch chan<- core.ProviderEvent) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)
	var (
		msgID            string
		currentEvent     string
		dataLines        []string
		indexToToolID    = make(map[int]string)
		indexToThinking  = make(map[int]bool)
	)

	flush := func() {
		if currentEvent == "" || len(dataLines) == 0 {
			return
		}
		data := strings.Join(dataLines, "")
		var ad anthropicSSEData
		if err := json.Unmarshal([]byte(data), &ad); err != nil {
			ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("parse sse: %w", err)}
			currentEvent = ""
			dataLines = nil
			return
		}

		switch currentEvent {
		case "message_start":
			msgID = ad.Message.ID
			ch <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}

		case "content_block_start":
			switch ad.ContentBlock.Type {
			case "thinking":
				indexToThinking[ad.Index] = true
			case "tool_use":
				toolCallID := ad.ContentBlock.ID
				indexToToolID[ad.Index] = toolCallID
				ch <- core.ProviderEvent{
					Type:       core.ProvToolCallStart,
					MessageID:  msgID,
					ToolCallID: toolCallID,
					ToolName:   ad.ContentBlock.Name,
				}
			}
			// text content_block_start: no event needed, deltas follow

		case "content_block_delta":
			switch ad.Delta.Type {
			case "thinking_delta":
				ch <- core.ProviderEvent{
					Type:         core.ProvThinkingDelta,
					MessageID:    msgID,
					ContentDelta: ad.Delta.Thinking,
				}
			case "redacted_thinking":
				ch <- core.ProviderEvent{
					Type:         core.ProvThinkingDelta,
					MessageID:    msgID,
					ContentDelta: "[redacted]",
				}
			case "text_delta":
				ch <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    msgID,
					ContentDelta: ad.Delta.Text,
				}
			case "input_json_delta":
				toolCallID := indexToToolID[ad.Index]
				ch <- core.ProviderEvent{
					Type:          core.ProvToolCallDelta,
					MessageID:     msgID,
					ToolCallID:    toolCallID,
					ToolArgsDelta: ad.Delta.PartialJSON,
				}
			}

		case "content_block_stop":
			if id, ok := indexToToolID[ad.Index]; ok {
				ch <- core.ProviderEvent{
					Type:       core.ProvToolCallEnd,
					MessageID:  msgID,
					ToolCallID: id,
				}
			}
			if indexToThinking[ad.Index] {
				ch <- core.ProviderEvent{
					Type:      core.ProvThinkingEnd,
					MessageID: msgID,
				}
				delete(indexToThinking, ad.Index)
			}

		case "message_delta":
			// Emit usage if present (requirement 5: token counting).
			if ad.Usage.InputTokens > 0 || ad.Usage.OutputTokens > 0 {
				ch <- core.ProviderEvent{
					Type: core.ProvUsage,
					Usage: &core.Usage{
						PromptTokens:     ad.Usage.InputTokens,
						CompletionTokens: ad.Usage.OutputTokens,
						TotalTokens:      ad.Usage.InputTokens + ad.Usage.OutputTokens,
					},
				}
			}

		case "message_stop":
			ch <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
		}

		currentEvent = ""
		dataLines = nil
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			flush()
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE scan: %w", err)}
	}
	flush()
}

// --- error classification (requirement 6) ------------------------------------

// classifyError returns a structured core.Error based on HTTP status code.
//
// Classification:
//   - 429 → Transient (rate limited)
//   - 400 → UsageError (bad request)
//   - 401/403 → Permanent (auth)
//   - 5xx → Transient (server error)
//   - default → Permanent
func (p *AnthropicMessagesProvider) classifyError(op string, statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &errResp)
	msg := errResp.Error.Message
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
