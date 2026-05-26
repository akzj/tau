//go:build !no_google
package google_genai

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

// Provider implements core.Provider for Google Generative AI (Gemini) API.
// Uses the Mihoyo gateway at ANTHROPIC_BASE_URL/v1beta/models/{model}:...
type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewProvider creates a Google GenAI provider using env vars:
//
//	ANTHROPIC_BASE_URL (default: https://athenai.mihoyo.com)
//	ANTHROPIC_AUTH_TOKEN (required)
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_AUTH_TOKEN not set")
	}
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://athenai.mihoyo.com"
	}
	return &Provider{
		baseURL: strings.TrimSuffix(baseURL, "/") + "/v1beta",
		apiKey:  apiKey,
		client:  &http.Client{},
	}, nil
}

// Stream implements core.Provider.Stream via SSE streamGenerateContent.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	body := p.buildStreamBody(req)

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

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", p.baseURL, req.Model.Name)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

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

// Complete implements core.Provider.Complete via non-streaming generateContent.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	body := p.buildCompleteBody(req)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return core.CompleteResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", p.baseURL, req.Model.Name)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return core.CompleteResponse{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

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
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return core.CompleteResponse{}, fmt.Errorf("decode response: %w", err)
	}

	content := ""
	if len(result.Candidates) > 0 {
		for _, part := range result.Candidates[0].Content.Parts {
			content += part.Text
		}
	}

	return core.CompleteResponse{
		Content: content,
		Usage: core.Usage{
			PromptTokens:     result.UsageMetadata.PromptTokenCount,
			CompletionTokens: result.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      result.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

// --- request body builders ---

func (p *Provider) buildStreamBody(req core.StreamRequest) map[string]any {
	body := map[string]any{
		"contents": buildContents(req.Messages),
		"generationConfig": map[string]any{
			"maxOutputTokens": 4096,
		},
	}
	if req.Options.Temperature > 0 {
		body["generationConfig"].(map[string]any)["temperature"] = req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["generationConfig"].(map[string]any)["maxOutputTokens"] = req.Options.MaxTokens
	}
	if req.SystemPrompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": req.SystemPrompt}},
		}
	}
	if len(req.Tools) > 0 {
		body["tools"] = buildToolDeclarations(req.Tools, req.TransformToolName)
	}
	return body
}

func (p *Provider) buildCompleteBody(req core.CompleteRequest) map[string]any {
	body := map[string]any{
		"contents": buildContents(req.Messages),
		"generationConfig": map[string]any{
			"maxOutputTokens": 4096,
		},
	}
	if req.Options.Temperature > 0 {
		body["generationConfig"].(map[string]any)["temperature"] = req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["generationConfig"].(map[string]any)["maxOutputTokens"] = req.Options.MaxTokens
	}
	if req.SystemPrompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": req.SystemPrompt}},
		}
	}
	return body
}

func buildToolDeclarations(tools []core.ToolSpec, transform func(string) string) []map[string]any {
	var fns []map[string]any
	for _, t := range tools {
		name := t.Name
		if transform != nil {
			name = transform(name)
		}
		fns = append(fns, map[string]any{
			"name":        name,
			"description": t.Description,
			"parameters":  t.Schema,
		})
	}
	return []map[string]any{{"functionDeclarations": fns}}
}

// buildContents converts tau Messages into Gemini contents array.
// Role mapping: core.RoleUser→"user", core.RoleAssistant→"model", core.RoleTool→"function".
// System messages are skipped (handled via systemInstruction).
func buildContents(msgs []core.Message) []map[string]any {
	// First pass: build a map of callID → toolName for functionResponse matching.
	callName := make(map[string]string)
	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			callName[tc.CallID] = tc.ToolName
		}
	}

	var contents []map[string]any
	for _, m := range msgs {
		if m.Role == core.RoleSystem {
			continue
		}

		role := string(m.Role)
		switch m.Role {
		case core.RoleAssistant:
			role = "model"
		case core.RoleTool:
			role = "function"
		}

		var parts []map[string]any

		// Assistant text content (omit if empty or if tool calls present without text)
		if m.Content != "" && m.Role != core.RoleTool {
			parts = append(parts, map[string]any{"text": m.Content})
		}

		// Tool calls from assistant
		for _, tc := range m.ToolCalls {
			var args map[string]any
			if tc.Args != "" {
				json.Unmarshal([]byte(tc.Args), &args)
			}
			if args == nil {
				args = map[string]any{}
			}
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{
					"name": tc.ToolName,
					"args": args,
				},
			})
		}

		// Tool result (function role)
		if m.Role == core.RoleTool {
			fnName := callName[m.ToolCallID]
			if fnName == "" {
				fnName = m.ToolCallID
			}
			parts = append(parts, map[string]any{
				"functionResponse": map[string]any{
					"name":     fnName,
					"response": map[string]any{"content": m.Content},
				},
			})
		}

		if len(parts) > 0 {
			contents = append(contents, map[string]any{"role": role, "parts": parts})
		}
	}
	return contents
}

// --- SSE parsing ---

// geminiChunk is a single SSE data line from Gemini streamGenerateContent.
type geminiChunk struct {
	Candidates []struct {
		Content struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string         `json:"name"`
					Args map[string]any `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

func (p *Provider) parseSSE(ctx context.Context, body io.ReadCloser, events chan<- core.ProviderEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	msgID := fmt.Sprintf("gemini-%d", time.Now().UnixNano())
	events <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk geminiChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE json: %w", err)}
			continue
		}

		for _, c := range chunk.Candidates {
			for _, part := range c.Content.Parts {
				if part.Text != "" {
					events <- core.ProviderEvent{
						Type:         core.ProvContentDelta,
						MessageID:    msgID,
						ContentDelta: part.Text,
					}
				}
				if part.FunctionCall != nil {
					callID := fmt.Sprintf("fc-%d", time.Now().UnixNano())
					events <- core.ProviderEvent{
						Type:       core.ProvToolCallStart,
						MessageID:  msgID,
						ToolCallID: callID,
						ToolName:   part.FunctionCall.Name,
					}
					argsJSON, _ := json.Marshal(part.FunctionCall.Args)
					events <- core.ProviderEvent{
						Type:          core.ProvToolCallDelta,
						MessageID:     msgID,
						ToolCallID:    callID,
						ToolArgsDelta: string(argsJSON),
					}
					events <- core.ProviderEvent{
						Type:       core.ProvToolCallEnd,
						MessageID:  msgID,
						ToolCallID: callID,
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE scan: %w", err)}
	}
	events <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
}