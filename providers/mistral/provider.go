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

	"github.com/akzj/tau/core"
)

// Provider implements core.Provider for Mistral (OpenAI-compatible).
type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewProvider creates a Mistral provider.
// Env vars:
//
//	MISTRAL_API_KEY (required, fallback: ANTHROPIC_AUTH_TOKEN)
//	MISTRAL_BASE_URL (default: https://api.mistral.ai/v1)
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_AUTH_TOKEN")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("MISTRAL_API_KEY not set")
	}
	baseURL := os.Getenv("MISTRAL_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.mistral.ai/v1"
	}
	return &Provider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{},
	}, nil
}

// Stream sends a chat completion request. API key is read from MISTRAL_API_KEY env var per call.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	body := p.buildBody(req)

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

// Complete sends a non-streaming completion request. API key is read per call.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	body := map[string]any{
		"messages": buildMessages(req.Messages, req.SystemPrompt),
		"stream":   false,
	}
	if req.Options.Temperature > 0 {
		body["temperature"] = req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	}

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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return core.CompleteResponse{}, fmt.Errorf("decode response: %w", err)
	}

	content := ""
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

// --- helpers ---

func (p *Provider) buildBody(req core.StreamRequest) map[string]any {
	body := map[string]any{
		"messages": buildMessages(req.Messages, req.SystemPrompt),
		"stream":   true,
	}
	if req.Options.Temperature > 0 {
		body["temperature"] = req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	}
	if len(req.Tools) > 0 {
		var tds []map[string]any
		for _, t := range req.Tools {
			name := t.Name
			if req.TransformToolName != nil {
				name = req.TransformToolName(name)
			}
			tds = append(tds, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        name,
					"description": t.Description,
					"parameters":  t.Schema,
				},
			})
		}
		body["tools"] = tds
	}
	return body
}

func (p *Provider) newRequest(ctx context.Context, body map[string]any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	return httpReq, nil
}

// --- SSE parsing ---

type mistralChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *Provider) parseSSE(ctx context.Context, body io.ReadCloser, events chan<- core.ProviderEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var (
		msgID            string
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
			break
		}

		var chunk mistralChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE json: %w", err)}
			continue
		}

		if msgID == "" && chunk.ID != "" {
			msgID = chunk.ID
			events <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
		}

		for _, choice := range chunk.Choices {
			delta := choice.Delta

			if delta.Content != "" {
				events <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    msgID,
					ContentDelta: delta.Content,
				}
			}

			for _, tc := range delta.ToolCalls {
				toolCallID := tc.ID
				if toolCallID == "" {
					if id, ok := indexToID[tc.Index]; ok {
						toolCallID = id
					} else {
						toolCallID = fmt.Sprintf("call-%d", tc.Index)
					}
				} else {
					indexToID[tc.Index] = toolCallID
				}

				if tc.Function.Name != "" {
					events <- core.ProviderEvent{
						Type:      core.ProvToolCallStart,
						MessageID: msgID,
						ToolCallID: toolCallID,
						ToolName:   tc.Function.Name,
					}
					startedToolCalls = append(startedToolCalls, toolCallID)
				}

				if tc.Function.Arguments != "" {
					events <- core.ProviderEvent{
						Type:          core.ProvToolCallDelta,
						MessageID:     msgID,
						ToolCallID:    toolCallID,
						ToolArgsDelta: tc.Function.Arguments,
					}
				}
			}

			if choice.FinishReason == "tool_calls" {
				for _, tcid := range startedToolCalls {
					events <- core.ProviderEvent{
						Type:       core.ProvToolCallEnd,
						MessageID:  msgID,
						ToolCallID: tcid,
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE scan: %w", err)}
	}

	if msgID != "" {
		events <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
	}
}

// --- message building ---

func buildMessages(messages []core.Message, systemPrompt string) []map[string]any {
	var result []map[string]any
	if systemPrompt != "" {
		result = append(result, map[string]any{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	for _, m := range messages {
		msg := map[string]any{
			"role":    string(m.Role),
			"content": m.Content,
		}
		if len(m.ToolCalls) > 0 {
			var tcs []map[string]any
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id":   tc.CallID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.ToolName,
						"arguments": tc.Args,
					},
				})
			}
			msg["tool_calls"] = tcs
			delete(msg, "content")
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		result = append(result, msg)
	}
	return result
}