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

	"github.com/akzj/tau/core"
)

// AnthropicMessagesProvider implements core.Provider for Anthropic Messages API.
type AnthropicMessagesProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewAnthropicMessagesProvider creates a provider for Anthropic Messages wire.
// Uses ANTHROPIC_AUTH_TOKEN and ANTHROPIC_BASE_URL env vars (same as OpenAI gateway).
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
		baseURL: strings.TrimSuffix(baseURL, "/") + "/v1",
		apiKey:  apiKey,
		client:  &http.Client{},
	}, nil
}

// Stream implements core.Provider.Stream.
func (p *AnthropicMessagesProvider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	body := p.buildAnthropicBody(req, true)

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

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if req.OnResponse != nil {
		req.OnResponse(resp.StatusCode, resp.Header)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	events := make(chan core.ProviderEvent, 64)
	go p.parseSSE(ctx, resp.Body, events)
	return events, nil
}

// Complete implements core.Provider.Complete.
func (p *AnthropicMessagesProvider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	body := p.buildAnthropicBody(core.StreamRequest{
		Messages:     req.Messages,
		SystemPrompt: req.SystemPrompt,
		Model:        req.Model,
	}, false)

	httpReq, err := p.newRequest(ctx, body)
	if err != nil {
		return core.CompleteResponse{}, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return core.CompleteResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return core.CompleteResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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

// --- request building ---

func (p *AnthropicMessagesProvider) buildAnthropicBody(req core.StreamRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":      req.Model.Name,
		"max_tokens": 4096,
		"stream":     stream,
	}
	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	}
	// system is a top-level field, not a message
	if req.SystemPrompt != "" {
		body["system"] = req.SystemPrompt
	}

	// messages in Anthropic content-block format
	body["messages"] = buildAnthropicMessages(req.Messages)

	// tools in Anthropic format (input_schema instead of parameters)
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, t := range req.Tools {
			name := t.Name
			if req.TransformToolName != nil {
				name = req.TransformToolName(name)
			}
			tools = append(tools, map[string]any{
				"name":         name,
				"description":  t.Description,
				"input_schema": t.Schema,
			})
		}
		body["tools"] = tools
	}

	return body
}

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
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	return httpReq, nil
}

// --- message building ---

func buildAnthropicMessages(messages []core.Message) []map[string]any {
	var msgs []map[string]any
	for _, m := range messages {
		// Skip system messages — system prompt is a top-level field
		if m.Role == core.RoleSystem {
			continue
		}

		msg := map[string]any{"role": string(m.Role)}

		switch {
		case m.Role == core.RoleAssistant && len(m.ToolCalls) > 0:
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
			msg = map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":        "tool_result",
					"tool_use_id": m.ToolCallID,
					"content":     m.Content,
				}},
			}

		default:
			msg["content"] = m.Content
		}

		msgs = append(msgs, msg)
	}
	return msgs
}

// --- SSE parsing ---

// anthropicEvent is a single SSE event line + data line.
type anthropicSSEEvent struct {
	eventType string
	data      string
}

// anthropicData is the JSON payload of an SSE event.
type anthropicData struct {
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

func (p *AnthropicMessagesProvider) parseSSE(ctx context.Context, body io.ReadCloser, events chan<- core.ProviderEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var (
		msgID          string
		currentEvent   string
		dataLines      []string
		indexToToolID  = make(map[int]string)
		indexToThinking = make(map[int]bool)
	)

	flush := func() {
		if currentEvent == "" || len(dataLines) == 0 {
			return
		}
		data := strings.Join(dataLines, "")
		var ad anthropicData
		if err := json.Unmarshal([]byte(data), &ad); err != nil {
			events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("parse sse: %w", err)}
			currentEvent = ""
			dataLines = nil
			return
		}

		switch currentEvent {
		case "message_start":
			msgID = ad.Message.ID
			events <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}

		case "content_block_start":
			switch ad.ContentBlock.Type {
			case "thinking":
				indexToThinking[ad.Index] = true
			case "tool_use":
				toolCallID := ad.ContentBlock.ID
				indexToToolID[ad.Index] = toolCallID
				events <- core.ProviderEvent{
					Type:      core.ProvToolCallStart,
					MessageID: msgID,
					ToolCallID: toolCallID,
					ToolName:   ad.ContentBlock.Name,
				}
			}
			// text content_block_start: ignore (text is empty)

		case "content_block_delta":
			switch ad.Delta.Type {
			case "thinking_delta":
				events <- core.ProviderEvent{
					Type:         core.ProvThinkingDelta,
					MessageID:    msgID,
					ContentDelta: ad.Delta.Thinking,
				}
			case "redacted_thinking":
				events <- core.ProviderEvent{
					Type:         core.ProvThinkingDelta,
					MessageID:    msgID,
					ContentDelta: "[redacted]",
				}
			case "text_delta":
				events <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    msgID,
					ContentDelta: ad.Delta.Text,
				}
			case "input_json_delta":
				toolCallID := indexToToolID[ad.Index]
				events <- core.ProviderEvent{
					Type:          core.ProvToolCallDelta,
					MessageID:     msgID,
					ToolCallID:    toolCallID,
					ToolArgsDelta: ad.Delta.PartialJSON,
				}
			}

		case "content_block_stop":
			// Emit ProvToolCallEnd for tool_use blocks at this index
			if id, ok := indexToToolID[ad.Index]; ok {
				events <- core.ProviderEvent{
					Type:       core.ProvToolCallEnd,
					MessageID:  msgID,
					ToolCallID: id,
				}
			}
			// Emit ProvThinkingEnd for thinking blocks at this index
			if indexToThinking[ad.Index] {
				events <- core.ProviderEvent{
					Type:      core.ProvThinkingEnd,
					MessageID: msgID,
				}
				delete(indexToThinking, ad.Index)
			}

		case "message_delta":
			// Emit ProvMessageEnd before message_stop
			events <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}

		case "message_stop":
			// Stream complete — nothing extra needed
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
			// Flush previous event if any
			flush()
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
		// Empty lines or comment lines are ignored
	}
	// Flush final event
	flush()
}

// --- compat ---

// Note: compat types are defined in core/provider.go as sealed WireCompat impls.

// Compat returns the wire-specific compatibility flags for this provider.
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