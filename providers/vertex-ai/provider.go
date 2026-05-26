//go:build !no_vertex

package vertex_ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// Provider implements core.Provider for Google Cloud Vertex AI.
type Provider struct {
	projectID   string
	location    string
	accessToken string
	client      *http.Client
}

// NewProvider creates a Vertex AI provider.
// Auth: VERTEX_ACCESS_TOKEN env var, or falls back to gcloud CLI.
// Env: VERTEX_PROJECT_ID (required), VERTEX_LOCATION (default: us-central1).
func NewProvider() (*Provider, error) {
	projectID := os.Getenv("VERTEX_PROJECT_ID")
	if projectID == "" {
		return nil, fmt.Errorf("VERTEX_PROJECT_ID not set")
	}
	location := os.Getenv("VERTEX_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	accessToken := os.Getenv("VERTEX_ACCESS_TOKEN")
	if accessToken == "" {
		out, err := exec.Command("gcloud", "auth", "print-access-token").Output()
		if err == nil && len(out) > 0 {
			accessToken = strings.TrimSpace(string(out))
		}
	}
	if accessToken == "" {
		return nil, fmt.Errorf("VERTEX_ACCESS_TOKEN not set and gcloud auth not available")
	}

	return &Provider{
		projectID:   projectID,
		location:    location,
		accessToken: accessToken,
		client:      &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Stream sends a streamGenerateContent request. API key is read per call.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	body := p.buildBody(req)

	httpReq, err := p.newRequest(ctx, req.Model.Name, ":streamGenerateContent?alt=sse", body)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
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

// Complete sends a non-streaming generateContent request. API key is read per call.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	body := p.buildCompleteBody(req)

	httpReq, err := p.newRequest(ctx, req.Model.Name, ":generateContent", body)
	if err != nil {
		return core.CompleteResponse{}, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return core.CompleteResponse{}, err
	}
	defer resp.Body.Close()

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

	var content string
	if len(result.Candidates) > 0 {
		for _, p := range result.Candidates[0].Content.Parts {
			content += p.Text
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

// --- helpers ---

func (p *Provider) endpoint(modelID, suffix string) string {
	return fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s%s",
		p.location, p.projectID, p.location, modelID, suffix,
	)
}

func (p *Provider) newRequest(ctx context.Context, modelID, suffix string, body map[string]any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.endpoint(modelID, suffix), bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.accessToken)
	return httpReq, nil
}

func (p *Provider) buildBody(req core.StreamRequest) map[string]any {
	body := map[string]any{
		"contents": buildContents(req.Messages),
	}
	if req.SystemPrompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": req.SystemPrompt}},
		}
	}
	if len(req.Tools) > 0 {
		var fns []map[string]any
		for _, t := range req.Tools {
			name := t.Name
			if req.TransformToolName != nil {
				name = req.TransformToolName(name)
			}
			fns = append(fns, map[string]any{
				"name":        name,
				"description": t.Description,
				"parameters":  t.Schema,
			})
		}
		body["tools"] = []map[string]any{{"functionDeclarations": fns}}
	}

	genConfig := map[string]any{}
	if req.Options.Temperature > 0 {
		genConfig["temperature"] = req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = req.Options.MaxTokens
	}
	if len(genConfig) > 0 {
		body["generationConfig"] = genConfig
	}
	return body
}

func (p *Provider) buildCompleteBody(req core.CompleteRequest) map[string]any {
	body := map[string]any{
		"contents": buildContents(req.Messages),
	}
	if req.SystemPrompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": req.SystemPrompt}},
		}
	}
	if req.Options.Temperature > 0 || req.Options.MaxTokens > 0 {
		genConfig := map[string]any{}
		if req.Options.Temperature > 0 {
			genConfig["temperature"] = req.Options.Temperature
		}
		if req.Options.MaxTokens > 0 {
			genConfig["maxOutputTokens"] = req.Options.MaxTokens
		}
		body["generationConfig"] = genConfig
	}
	return body
}

// --- SSE parsing ---

func (p *Provider) parseSSE(ctx context.Context, body io.ReadCloser, events chan<- core.ProviderEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var (
		msgID        = fmt.Sprintf("msg-%d", time.Now().UnixNano())
		started      bool
		callID       string
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

		var chunk struct {
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

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("SSE json: %w", err)}
			continue
		}

		if !started {
			started = true
			events <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
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
					callID = fmt.Sprintf("fc-%d", time.Now().UnixNano())
					events <- core.ProviderEvent{
						Type:      core.ProvToolCallStart,
						MessageID: msgID,
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

// --- message building ---

func buildContents(messages []core.Message) []map[string]any {
	var contents []map[string]any
	for _, m := range messages {
		// Skip system messages — systemInstruction is a top-level field
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

		// Text content
		if m.Content != "" {
			parts = append(parts, map[string]any{"text": m.Content})
		}

		// Tool calls from assistant
		for _, tc := range m.ToolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tc.Args), &args)
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

		// Tool result
		if m.Role == core.RoleTool {
			parts = append(parts, map[string]any{
				"functionResponse": map[string]any{
					"name":     "unknown",
					"response": map[string]any{"content": m.Content},
				},
			})
		}

		contents = append(contents, map[string]any{"role": role, "parts": parts})
	}
	return contents
}