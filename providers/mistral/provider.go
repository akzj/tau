//go:build !no_mistral

package mistral

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

const defaultMistralBaseURL = "https://api.mistral.ai/v1"

// Provider implements core.Provider for Mistral.
// Mistral is OpenAI-compatible and uses Bearer token authentication.
type Provider struct {
	baseURL   string
	apiKey    string
	client    *http.Client
	maxTokens int
}

// New creates a provider with an explicit API key (used in tests).
func New(apiKey string) *Provider {
	return &Provider{
		baseURL:   defaultMistralBaseURL,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 120 * time.Second},
		maxTokens: 4096,
	}
}

// NewProvider creates a Mistral provider from environment variables.
//
// Env vars:
//
//	MISTRAL_API_KEY (required) — API key
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("MISTRAL_API_KEY not set")
	}
	return &Provider{
		baseURL:   defaultMistralBaseURL,
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
		return core.CompleteResponse{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return core.CompleteResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return core.CompleteResponse{}, p.classifyError("mistral.Complete", resp.StatusCode, bodyBytes)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return core.CompleteResponse{}, fmt.Errorf("decode response: %w", err)
	}

	var content string
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	return core.CompleteResponse{
		Content: content,
		Usage: core.Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

// Stream sends a streaming chat completion request.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
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

	resp, err := p.client.Do(httpReq)
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
		return nil, p.classifyError("mistral.Stream", resp.StatusCode, bodyBytes)
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
		TemperatureField:      true,
		TopPField:             true,
		SupportsStop:          true,
		SupportsStreamOptions: true,
		ResponseFormatField:   true,
		MaxTokensField:        true,
	}
}

// --- Request building --------------------------------------------------------

// buildRequest constructs the Mistral Chat Completions API request body.
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
	if req.Options.TopP > 0 {
		body["top_p"] = req.Options.TopP
	}
	if len(req.Options.Stop) > 0 {
		body["stop"] = req.Options.Stop
	}

	if len(req.Tools) > 0 {
		body["tools"] = convertTools(req.Tools, req.TransformToolName)
		body["tool_choice"] = "auto"
	}

	return body
}

// newRequest creates an HTTP request with Mistral headers.
// Mistral uses Authorization: Bearer like OpenAI.
func (p *Provider) newRequest(ctx context.Context, body map[string]any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	return httpReq, nil
}

// --- Message conversion ------------------------------------------------------

// contentBlock is the parsed JSON content block for vision messages.
type contentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text"`
	Source *imageSource `json:"source"`
}

// imageSource is the base64 image source for vision content blocks.
type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// convertMessages converts core.Message slice to OpenAI Chat Completions format.
//
// Supports:
//   - Plain text messages
//   - Vision content blocks (Anthropic-style JSON → OpenAI image_url format)
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
			// Check for vision content blocks (JSON-encoded in Content field).
			if blocks := parseVisionBlocks(msg.Content); blocks != nil {
				m["content"] = blocks
			} else {
				m["content"] = msg.Content
			}
		}

		out = append(out, m)
	}

	return out
}

// parseVisionBlocks attempts to parse s as a JSON array of contentBlock.
// Converts Anthropic-style image blocks to OpenAI image_url format.
// Returns nil if parsing fails — caller should treat s as plain text.
func parseVisionBlocks(s string) []map[string]any {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return nil
	}

	var blocks []contentBlock
	if err := json.Unmarshal([]byte(s), &blocks); err != nil {
		return nil
	}

	var out []map[string]any
	for _, b := range blocks {
		switch b.Type {
		case "text":
			out = append(out, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		case "image":
			if b.Source != nil {
				out = append(out, map[string]any{
					"type": "image_url",
					"image_url": map[string]string{
						"url": fmt.Sprintf("data:%s;base64,%s", b.Source.MediaType, b.Source.Data),
					},
				})
			}
		}
	}
	return out
}

// --- Tool conversion ---------------------------------------------------------

// convertTools converts core.ToolSpec slice to OpenAI function calling format.
func convertTools(tools []core.ToolSpec, transform func(string) string) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		name := t.Name
		if transform != nil {
			name = transform(name)
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": t.Description,
				"parameters":  t.Schema,
			},
		})
	}
	return out
}

// --- SSE parsing -------------------------------------------------------------

// mistralSSEChunk is the JSON payload of a Mistral SSE data line.
type mistralSSEChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// parseSSE reads Mistral SSE streaming responses and emits ProviderEvents.
func (p *Provider) parseSSE(ctx context.Context, body io.ReadCloser, ch chan<- core.ProviderEvent) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)
	var (
		msgID            string
		finished         bool
		startedToolCalls []string
		indexToID        = make(map[int]string)
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
		if data == "[DONE]" {
			finished = true
			if msgID != "" {
				ch <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
			}
			break
		}

		var chunk mistralSSEChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("parse chunk: %w", err)}
			continue
		}

		// Track message ID on first chunk.
		if msgID == "" && chunk.ID != "" {
			msgID = chunk.ID
			ch <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
		}

		// Emit usage if present.
		if chunk.Usage != nil {
			ch <- core.ProviderEvent{
				Type: core.ProvUsage,
				Usage: &core.Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				},
			}
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta

			// Text content delta.
			if delta.Content != "" {
				ch <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    msgID,
					ContentDelta: delta.Content,
				}
			}

			// Tool call deltas.
			for _, tc := range delta.ToolCalls {
				toolCallID := tc.ID
				if toolCallID != "" {
					// New tool call: register ID → index mapping.
					indexToID[tc.Index] = toolCallID
				} else {
					// Continuation: look up ID by index.
					if id, ok := indexToID[tc.Index]; ok {
						toolCallID = id
					}
				}

				// Tool call start: has both ID and name.
				if tc.ID != "" && tc.Function.Name != "" {
					ch <- core.ProviderEvent{
						Type:       core.ProvToolCallStart,
						MessageID:  msgID,
						ToolCallID: toolCallID,
						ToolName:   tc.Function.Name,
					}
					startedToolCalls = append(startedToolCalls, toolCallID)
				}

				// Tool argument deltas.
				if tc.Function.Arguments != "" {
					ch <- core.ProviderEvent{
						Type:          core.ProvToolCallDelta,
						MessageID:     msgID,
						ToolCallID:    toolCallID,
						ToolArgsDelta: tc.Function.Arguments,
					}
				}
			}

			// Finish reason.
			if choice.FinishReason != nil {
				fr := *choice.FinishReason
				// End all pending tool calls on tool_calls or stop.
				if fr == "tool_calls" || fr == "stop" {
					for _, tcid := range startedToolCalls {
						ch <- core.ProviderEvent{
							Type:       core.ProvToolCallEnd,
							MessageID:  msgID,
							ToolCallID: tcid,
						}
					}
					startedToolCalls = nil
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE scan: %w", err)}
	}

	// Emit final message end if we haven't already.
	if !finished && msgID != "" {
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
func (p *Provider) classifyError(op string, statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
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