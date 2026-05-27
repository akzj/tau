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

const genaiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// Provider implements core.Provider for the Google Generative AI (Gemini) API.
//
// Requirements:
//  1. Full POST /v1beta/models/{model}:generateContent + :streamGenerateContent?alt=sse.
//  2. SSE streaming via data: {...} → core.ProviderEvent channel (delta-computed from accumulated text).
//  3. Vision: inlineData content blocks with base64 data.
//  4. Function calling: functionDeclarations → functionCall in response.
//  5. Token usage: usageMetadata{} parsed from complete + streaming.
//  6. Error classification: 429→Transient, 400→Usage, 401/403→Permanent, 500+→Transient.
//  7. Backward compat: Compat() string, CountTokens(), OnPayload/OnResponse hooks.
type Provider struct {
	baseURL   string
	apiKey    string
	client    *http.Client
	maxTokens int
}

// New creates a provider with an explicit API key (used in tests).
func New(apiKey string) *Provider {
	return &Provider{
		baseURL:   genaiBaseURL,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 120 * time.Second},
		maxTokens: 4096,
	}
}

// NewProvider creates a provider from environment variables (backward compat).
func NewProvider() (*Provider, error) { return NewGoogleGenAIProvider() }

// NewGoogleGenAIProvider creates a provider from environment variables.
func NewGoogleGenAIProvider() (*Provider, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY not set")
	}
	baseURL := os.Getenv("GOOGLE_GENAI_BASE_URL")
	if baseURL == "" {
		baseURL = genaiBaseURL
	}
	return &Provider{
		baseURL:   baseURL,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 120 * time.Second},
		maxTokens: 4096,
	}, nil
}

// --- core.Provider implementation --------------------------------------------

// Complete sends a non-streaming generateContent request.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	body := p.buildRequest(core.StreamRequest{
		Model:        req.Model,
		Messages:     req.Messages,
		SystemPrompt: req.SystemPrompt,
		Options:      req.Options,
	})

	url := fmt.Sprintf("%s/%s:generateContent", p.baseURL, req.Model.Name)
	httpReq, err := p.newRequest(ctx, url, body)
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
		return core.CompleteResponse{}, p.classifyError("genai.Complete", resp.StatusCode, bodyBytes)
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
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return core.CompleteResponse{}, fmt.Errorf("decode response: %w", err)
	}

	var content string
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

// Stream sends a streaming generateContent request (SSE).
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	body := p.buildRequest(req)

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

	url := fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse", p.baseURL, req.Model.Name)
	httpReq, err := p.newRequest(ctx, url, body)
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
		return nil, p.classifyError("genai.Stream", resp.StatusCode, bodyBytes)
	}

	ch := make(chan core.ProviderEvent, 64)
	go p.parseSSE(ctx, resp.Body, ch)
	return ch, nil
}

// CountTokens estimates the number of tokens in a text.
func (p *Provider) CountTokens(text string) int {
	return len(text) / 4
}

// Compat returns the provider identifier string.
func (p *Provider) Compat() string {
	return "google-genai"
}

// --- Request building --------------------------------------------------------

// buildRequest constructs the Gemini API request body.
//
// Requirements:
//  1. Full POST body: contents, systemInstruction, tools, toolConfig, generationConfig.
//  3. Vision: inlineData parts converted from content blocks.
//  4. Tool calls: functionDeclarations[] with functionCallingConfig mode=AUTO.
func (p *Provider) buildRequest(req core.StreamRequest) map[string]any {
	body := map[string]any{
		"contents": convertContents(req.Messages),
	}

	if req.SystemPrompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": req.SystemPrompt}},
		}
	}

	if len(req.Tools) > 0 {
		body["tools"] = convertTools(req.Tools, req.TransformToolName)
		body["toolConfig"] = map[string]any{
			"functionCallingConfig": map[string]string{"mode": "AUTO"},
		}
	}

	genConfig := map[string]any{}
	if req.Options.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = req.Options.MaxTokens
	} else {
		genConfig["maxOutputTokens"] = p.maxTokens
	}
	if req.Options.Temperature > 0 {
		genConfig["temperature"] = req.Options.Temperature
	}
	if req.Options.TopP > 0 {
		genConfig["topP"] = req.Options.TopP
	}
	if len(req.Options.Stop) > 0 {
		genConfig["stopSequences"] = req.Options.Stop
	}
	if len(genConfig) > 0 {
		body["generationConfig"] = genConfig
	}

	return body
}

// newRequest creates an HTTP request with required headers.
func (p *Provider) newRequest(ctx context.Context, url string, body map[string]any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)
	return httpReq, nil
}

// --- Message conversion (requirement 3: vision) ------------------------------

// contentBlock is the parsed JSON content block for vision/data messages.
type contentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *imageSource `json:"source,omitempty"`
}

// imageSource is the base64 image source for vision content blocks.
type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// convertContents converts core.Message slice to Gemini contents array.
//
// Supports:
//   - Plain text messages
//   - Vision content blocks (Anthropic-style JSON → Gemini inlineData format)
//   - Assistant functionCall parts
//   - Tool result functionResponse parts
func convertContents(msgs []core.Message) []map[string]any {
	// Build lookup of callID → toolName for functionResponse matching.
	callName := make(map[string]string)
	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			callName[tc.CallID] = tc.ToolName
		}
	}

	var out []map[string]any
	for _, m := range msgs {
		// System messages are handled via systemInstruction.
		if m.Role == core.RoleSystem {
			continue
		}

		role := "user"
		switch m.Role {
		case core.RoleAssistant:
			role = "model"
		case core.RoleTool:
			role = "function"
		}

		var parts []map[string]any

		// Check for vision content blocks (JSON-encoded in Content field).
		if blocks := parseContentBlocks(m.Content); blocks != nil {
			parts = blocks
		} else if m.Content != "" && m.Role != core.RoleTool {
			parts = append(parts, map[string]any{"text": m.Content})
		}

		// Tool calls from assistant → functionCall parts.
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

		// Tool result → functionResponse part.
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
			out = append(out, map[string]any{"role": role, "parts": parts})
		}
	}
	return out
}

// parseContentBlocks attempts to parse s as a JSON array of contentBlock.
// Converts Anthropic-style image blocks to Gemini inlineData format.
// Returns nil if parsing fails — caller should treat s as plain text.
func parseContentBlocks(s string) []map[string]any {
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
			out = append(out, map[string]any{"text": b.Text})
		case "image":
			if b.Source != nil {
				out = append(out, map[string]any{
					"inlineData": map[string]string{
						"mimeType": b.Source.MediaType,
						"data":     b.Source.Data,
					},
				})
			}
		}
	}
	return out
}

// --- Tool conversion (requirement 4: function calling) -----------------------

// convertTools converts core.ToolSpec slice to Gemini functionDeclarations format.
func convertTools(tools []core.ToolSpec, transform func(string) string) []map[string]any {
	var funcDecls []map[string]any
	for _, t := range tools {
		name := t.Name
		if transform != nil {
			name = transform(name)
		}
		funcDecls = append(funcDecls, map[string]any{
			"name":        name,
			"description": t.Description,
			"parameters":  t.Schema,
		})
	}
	return []map[string]any{{"functionDeclarations": funcDecls}}
}

// --- SSE parsing (requirement 2: streaming, requirement 5: token usage) ------

// genaiSSEChunk is a single SSE data line from Gemini streamGenerateContent.
// Google GenAI returns complete response objects (not deltas) — text accumulates.
type genaiSSEChunk struct {
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
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// parseSSE reads Gemini SSE streaming responses and emits ProviderEvents.
// Google GenAI returns accumulated text in each chunk, so we compute deltas.
func (p *Provider) parseSSE(ctx context.Context, body io.ReadCloser, ch chan<- core.ProviderEvent) {
	defer body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(body)
	msgID := fmt.Sprintf("genai-%d", time.Now().UnixNano())

	var (
		started    bool
		finished   bool
		prevText   string
	)

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

		var chunk genaiSSEChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE parse: %w", err)}
			continue
		}

		// Emit message start on first chunk.
		if !started {
			started = true
			ch <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
		}

		// Usage metadata (requirement 5: token usage in streaming).
		if chunk.UsageMetadata != nil {
			ch <- core.ProviderEvent{
				Type: core.ProvUsage,
				Usage: &core.Usage{
					PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
					CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
					TotalTokens:      chunk.UsageMetadata.TotalTokenCount,
				},
			}
		}

		for _, cand := range chunk.Candidates {
			for _, part := range cand.Content.Parts {
				// Text delta: compute from accumulated text.
				if part.Text != "" {
					if strings.HasPrefix(part.Text, prevText) {
						delta := part.Text[len(prevText):]
						prevText = part.Text
						if delta != "" {
							ch <- core.ProviderEvent{
								Type:         core.ProvContentDelta,
								MessageID:    msgID,
								ContentDelta: delta,
							}
						}
					} else {
						// Non-contiguous text — emit the full text as delta.
						ch <- core.ProviderEvent{
							Type:         core.ProvContentDelta,
							MessageID:    msgID,
							ContentDelta: part.Text,
						}
						prevText = part.Text
					}
				}

				// Function call: complete in one part.
				if part.FunctionCall != nil {
					callID := fmt.Sprintf("fc-%d", time.Now().UnixNano())
					ch <- core.ProviderEvent{
						Type:       core.ProvToolCallStart,
						MessageID:  msgID,
						ToolCallID: callID,
						ToolName:   part.FunctionCall.Name,
					}
					argsJSON, _ := json.Marshal(part.FunctionCall.Args)
					ch <- core.ProviderEvent{
						Type:          core.ProvToolCallDelta,
						MessageID:     msgID,
						ToolCallID:    callID,
						ToolArgsDelta: string(argsJSON),
					}
					ch <- core.ProviderEvent{
						Type:       core.ProvToolCallEnd,
						MessageID:  msgID,
						ToolCallID: callID,
					}
				}
			}

			// Finish reason.
			if cand.FinishReason == "STOP" && !finished {
				finished = true
				ch <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE scan: %w", err)}
	}

	// Ensure message end is emitted if not already done.
	if !finished && started {
		ch <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
	}
}

// --- Error classification (requirement 6) ------------------------------------

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
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
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