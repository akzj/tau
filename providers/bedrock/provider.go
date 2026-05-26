//go:build !no_bedrock

package bedrock

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/akzj/tau/core"
)

// Provider implements core.Provider for AWS Bedrock (Anthropic Messages format).
type Provider struct {
	region       string
	accessKey    string
	secretKey    string
	sessionToken string
	signer       *v4.Signer
	client       *http.Client
}

// NewProvider creates a Bedrock provider using standard AWS credentials.
// Env vars: AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN.
func NewProvider() (*Provider, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}

	return &Provider{
		region:       region,
		accessKey:    accessKey,
		secretKey:    secretKey,
		sessionToken: os.Getenv("AWS_SESSION_TOKEN"),
		signer:       v4.NewSigner(),
		client:       &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Stream sends a Bedrock invoke-with-response-stream request.
// API key is read from AWS credentials env vars per call.
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

	modelID := req.Model.Name
	url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke-with-response-stream",
		p.region, modelID)

	httpReq, err := p.newSignedRequest(ctx, http.MethodPost, url, body)
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
	go p.parseJSONLines(ctx, resp.Body, events)
	return events, nil
}

// Complete sends a non-streaming Bedrock invoke request. API key is read per call.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	modelID := req.Model.Name
	url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke",
		p.region, modelID)

	body := p.buildAnthropicBody(req.Messages, req.SystemPrompt, req.Options)

	httpReq, err := p.newSignedRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return core.CompleteResponse{}, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return core.CompleteResponse{}, err
	}
	defer resp.Body.Close()

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

// --- helpers ---

func (p *Provider) buildBody(req core.StreamRequest) map[string]any {
	msgs := buildAnthropicMessages(req.Messages)
	body := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        4096,
		"messages":          msgs,
		"stream":            true,
	}
	if req.SystemPrompt != "" {
		body["system"] = req.SystemPrompt
	}
	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	}
	if req.Options.Temperature > 0 {
		body["temperature"] = req.Options.Temperature
	}
	if len(req.Tools) > 0 {
		var tds []map[string]any
		for _, t := range req.Tools {
			name := t.Name
			if req.TransformToolName != nil {
				name = req.TransformToolName(name)
			}
			tds = append(tds, map[string]any{
				"name":         name,
				"description":  t.Description,
				"input_schema": t.Schema,
			})
		}
		body["tools"] = tds
	}
	return body
}

func (p *Provider) buildAnthropicBody(messages []core.Message, systemPrompt string, opts core.ProviderOptions) map[string]any {
	msgs := buildAnthropicMessages(messages)
	body := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        4096,
		"messages":          msgs,
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}
	return body
}

func (p *Provider) newSignedRequest(ctx context.Context, method, url string, body map[string]any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Compute payload hash for SigV4
	hash := sha256.Sum256(jsonBody)
	payloadHash := hex.EncodeToString(hash[:])

	creds := aws.Credentials{
		AccessKeyID:     p.accessKey,
		SecretAccessKey: p.secretKey,
		SessionToken:    p.sessionToken,
	}
	if err := p.signer.SignHTTP(ctx, creds, httpReq, payloadHash, "bedrock", p.region, time.Now()); err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	return httpReq, nil
}

// --- JSON Lines parsing ---

func (p *Provider) parseJSONLines(ctx context.Context, body io.ReadCloser, events chan<- core.ProviderEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var (
		msgID            string
		indexToToolID    = make(map[int]string)
		startedToolCalls []string
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			ContentBlock struct {
				Type string `json:"type"`
				Text string `json:"text"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Index int `json:"index"`
			Message struct {
				ID         string `json:"id"`
				StopReason string `json:"stop_reason"`
			} `json:"message"`
		}

		if err := json.Unmarshal([]byte(line), &event); err != nil {
			events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("JSON line: %w", err)}
			continue
		}

		switch event.Type {
		case "message_start":
			msgID = event.Message.ID
			events <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
			indexToToolID = make(map[int]string)
			startedToolCalls = nil

		case "content_block_start":
			switch event.ContentBlock.Type {
			case "tool_use":
				toolCallID := event.ContentBlock.ID
				indexToToolID[event.Index] = toolCallID
				events <- core.ProviderEvent{
					Type:      core.ProvToolCallStart,
					MessageID: msgID,
					ToolCallID: toolCallID,
					ToolName:   event.ContentBlock.Name,
				}
				startedToolCalls = append(startedToolCalls, toolCallID)
			}

		case "content_block_delta":
			switch event.Delta.Type {
			case "text_delta":
				events <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    msgID,
					ContentDelta: event.Delta.Text,
				}
			case "input_json_delta":
				toolCallID := indexToToolID[event.Index]
				events <- core.ProviderEvent{
					Type:          core.ProvToolCallDelta,
					MessageID:     msgID,
					ToolCallID:    toolCallID,
					ToolArgsDelta: event.Delta.PartialJSON,
				}
			}

		case "content_block_stop":
			if id, ok := indexToToolID[event.Index]; ok {
				events <- core.ProviderEvent{
					Type:       core.ProvToolCallEnd,
					MessageID:  msgID,
					ToolCallID: id,
				}
			}

		case "message_stop":
			events <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- core.ProviderEvent{Type: core.ProvError, Err: fmt.Errorf("scan: %w", err)}
	}
}

// --- message building ---

func buildAnthropicMessages(msgs []core.Message) []map[string]any {
	var result []map[string]any
	for _, m := range msgs {
		// Skip system messages — system prompt is a top-level field
		if m.Role == core.RoleSystem {
			continue
		}

		msg := map[string]any{"role": string(m.Role)}

		switch {
		case m.Role == core.RoleAssistant && len(m.ToolCalls) > 0:
			var content []map[string]any
			if m.Content != "" {
				content = append(content, map[string]any{"type": "text", "text": m.Content})
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
			msg["role"] = "user"
			msg["content"] = []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": m.ToolCallID,
				"content":     m.Content,
			}}

		default:
			msg["content"] = m.Content
		}

		result = append(result, msg)
	}
	return result
}