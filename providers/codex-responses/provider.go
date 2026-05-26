//go:build !no_codex

package codex_responses

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

// Provider implements core.Provider for Codex Responses API.
type Provider struct {
	baseURL string
	apiKey  string
	orgID   string
	client  *http.Client
}

// NewProvider creates a Codex Responses provider.
// Env vars: ANTHROPIC_BASE_URL (default: https://athenai.mihoyo.com), ANTHROPIC_AUTH_TOKEN (required), CODEX_ORG_ID (optional).
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_AUTH_TOKEN not set")
	}
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://athenai.mihoyo.com"
	}
	orgID := os.Getenv("CODEX_ORG_ID")
	return &Provider{
		baseURL: strings.TrimSuffix(baseURL, "/") + "/v1",
		apiKey:  apiKey,
		orgID:   orgID,
		client:  &http.Client{},
	}, nil
}

// Stream sends a Codex Responses API request. API key is read per call.
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
		b := string(bodyBytes)
		if resp.StatusCode == 429 {
			return nil, fmt.Errorf("rate limited (429): %s", b)
		}
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, b)
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("auth error (%d): %s", resp.StatusCode, b)
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, b)
	}

	events := make(chan core.ProviderEvent, 64)
	go p.parseSSE(ctx, resp.Body, events)
	return events, nil
}

// Complete sends a non-streaming Codex Responses request. API key is read per call.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	body := map[string]any{
		"model": req.Model.Name,
		"input": messagesToText(req.Messages),
	}
	if req.Options.Temperature > 0 {
		body["temperature"] = req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["max_output_tokens"] = req.Options.MaxTokens
	}

	httpReq, err := p.newRequest(ctx, body)
	if err != nil {
		return core.CompleteResponse{}, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return core.CompleteResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		b := string(bodyBytes)
		if resp.StatusCode == 429 {
			return core.CompleteResponse{}, fmt.Errorf("rate limited (429): %s", b)
		}
		if resp.StatusCode >= 500 {
			return core.CompleteResponse{}, fmt.Errorf("server error (%d): %s", resp.StatusCode, b)
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return core.CompleteResponse{}, fmt.Errorf("auth error (%d): %s", resp.StatusCode, b)
		}
		return core.CompleteResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, b)
	}
	defer resp.Body.Close()

	var result struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return core.CompleteResponse{}, fmt.Errorf("decode response: %w", err)
	}

	var text string
	for _, o := range result.Output {
		for _, c := range o.Content {
			text += c.Text
		}
	}

	return core.CompleteResponse{
		Content: text,
		Usage: core.Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

func (p *Provider) buildBody(req core.StreamRequest) map[string]any {
	body := map[string]any{
		"model":  req.Model.Name,
		"stream": true,
	}
	body["input"] = messagesToText(req.Messages)

	if req.SystemPrompt != "" {
		body["instructions"] = req.SystemPrompt
	}
	if req.Options.Temperature > 0 {
		body["temperature"] = req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["max_output_tokens"] = req.Options.MaxTokens
	}
	if len(req.Tools) > 0 {
		var tds []map[string]any
		for _, t := range req.Tools {
			name := t.Name
			if req.TransformToolName != nil {
				name = req.TransformToolName(name)
			}
			tds = append(tds, map[string]any{
				"type":        "function",
				"name":        name,
				"description": t.Description,
				"parameters":  t.Schema,
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
		p.baseURL+"/responses", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.orgID != "" {
		httpReq.Header.Set("OpenAI-Organization", p.orgID)
	}
	return httpReq, nil
}

// --- SSE parsing ---

func (p *Provider) parseSSE(ctx context.Context, body io.ReadCloser, events chan<- core.ProviderEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var msgID string

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

		var event struct {
			Type     string `json:"type"`
			Response struct {
				ID     string `json:"id"`
				Output []struct {
					Type      string `json:"type"`
					Content   []struct {
						Text string `json:"text"`
					} `json:"content"`
					Name      string `json:"name"`
					ID        string `json:"id"`
					Arguments string `json:"arguments"`
				} `json:"output"`
			} `json:"response"`
			Delta string `json:"delta"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE json: %w", err)}
			continue
		}

		switch event.Type {
		case "response.created":
			msgID = event.Response.ID
			events <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}

		case "response.output_text.delta":
			events <- core.ProviderEvent{
				Type:         core.ProvContentDelta,
				MessageID:    msgID,
				ContentDelta: event.Delta,
			}

		case "response.completed":
			for _, o := range event.Response.Output {
				if o.Type == "function_call" {
					events <- core.ProviderEvent{
						Type:      core.ProvToolCallStart,
						MessageID: msgID,
						ToolCallID: o.ID,
						ToolName:   o.Name,
					}
					if o.Arguments != "" {
						events <- core.ProviderEvent{
							Type:          core.ProvToolCallDelta,
							MessageID:     msgID,
							ToolCallID:    o.ID,
							ToolArgsDelta: o.Arguments,
						}
					}
					events <- core.ProviderEvent{
						Type:       core.ProvToolCallEnd,
						MessageID:  msgID,
						ToolCallID: o.ID,
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

func messagesToText(msgs []core.Message) string {
	var parts []string
	for _, m := range msgs {
		parts = append(parts, string(m.Role)+": "+m.Content)
	}
	return strings.Join(parts, "\n")
}