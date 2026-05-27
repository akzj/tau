// Package faux provides a test Provider with pre-queued responses and chaos-testing capabilities.
// Zero mock layers above the Provider — tests exercise the real Loop/Hook/Session stack.
package faux

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/akzj/tau/core"
)

// ---------------------------------------------------------------------------
// Chaos configuration
// ---------------------------------------------------------------------------

// Mode defines the chaos simulation mode.
type Mode string

const (
	ModeNormal Mode = "normal"
	ModeSlow   Mode = "slow"
	ModeFlaky  Mode = "flaky"
	ModeEvil   Mode = "evil"
	ModeDown   Mode = "down"
)

// Config configures the faux provider's chaos behaviour.
type Config struct {
	Mode       Mode          // chaos mode (default: normal)
	Latency    time.Duration // fixed latency added before every request (0 = none)
	LatencyMin time.Duration // minimum random-latency bound
	LatencyMax time.Duration // maximum random-latency bound
	ErrorRate  float64       // 0.0–1.0 probability of synthetic error per request
	MaxTokens  int           // simulated token budget (0 = unlimited)
	RecordFile string        // record-to file path (empty = no recording)
	ReplayFile string        // replay-from file path (empty = no replay)
}

// DefaultConfig reads Config from environment variables.
func DefaultConfig() Config {
	c := Config{Mode: ModeNormal, MaxTokens: 4096}
	if mode := os.Getenv("FAUX_MODE"); mode != "" {
		c.Mode = Mode(mode)
	}
	if lat := os.Getenv("FAUX_LATENCY"); lat != "" {
		c.Latency, _ = time.ParseDuration(lat)
	}
	if min := os.Getenv("FAUX_LATENCY_MIN"); min != "" {
		c.LatencyMin, _ = time.ParseDuration(min)
	}
	if max := os.Getenv("FAUX_LATENCY_MAX"); max != "" {
		c.LatencyMax, _ = time.ParseDuration(max)
	}
	if rate := os.Getenv("FAUX_ERROR_RATE"); rate != "" {
		fmt.Sscanf(rate, "%f", &c.ErrorRate)
	}
	if tok := os.Getenv("FAUX_MAX_TOKENS"); tok != "" {
		fmt.Sscanf(tok, "%d", &c.MaxTokens)
	}
	if rf := os.Getenv("FAUX_RECORD_FILE"); rf != "" {
		c.RecordFile = rf
	}
	if rpf := os.Getenv("FAUX_REPLAY_FILE"); rpf != "" {
		c.ReplayFile = rpf
	}
	return c
}

// ---------------------------------------------------------------------------
// Response templates
// ---------------------------------------------------------------------------

// ResponseTemplate defines a named response pattern for template-based replies.
type ResponseTemplate struct {
	Name            string
	Pattern         string // {{.Prompt}} is replaced with the first user message
	TokensPerSecond int
}

var defaultTemplates = map[string]ResponseTemplate{
	"coding": {
		Name: "coding",
		Pattern: "Here's the code for:\n\n```go\n// {{.Prompt}}\nfunc example() {\n\t// TODO: implement\n}\n```\n\nThis should work.",
		TokensPerSecond: 20,
	},
	"reasoning": {
		Name: "reasoning",
		Pattern: "Let me think about: {{.Prompt}}\n\n1. Requirements analysis.\n2. Trade-off evaluation.\n3. Recommended approach.\n\nBased on this, I suggest...",
		TokensPerSecond: 15,
	},
	"refusal": {
		Name: "refusal",
		Pattern: "I cannot help with: {{.Prompt}}. This request violates safety guidelines.",
		TokensPerSecond: 30,
	},
	"tool_use": {
		Name: "tool_use",
		Pattern: "I'll use a tool to answer: {{.Prompt}}",
		TokensPerSecond: 5,
	},
}

// ---------------------------------------------------------------------------
// Record-replay
// ---------------------------------------------------------------------------

// RecordedEvent is a timestamped provider event for record-replay.
type RecordedEvent struct {
	Time  time.Time          `json:"time"`
	Event core.ProviderEvent `json:"event"`
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

// Provider implements core.Provider with pre-queued response sequences
// and chaos-testing capabilities. Thread-safe.
type Provider struct {
	mu          sync.Mutex
	streams     []StreamResponse
	streamIdx   int
	completes   []CompleteEntry
	completeIdx int
	sessions    map[string]*cacheState
	factory     func(context.Context, core.StreamRequest) []StreamResponse
	config      Config
	templates   map[string]ResponseTemplate
	recorded    []RecordedEvent
}

type cacheState struct {
	commonPrefix int
	cacheReads   []int
	cacheWrites  []int
}

// StreamResponse is a pre-queued streaming response.
type StreamResponse struct {
	Events          []core.ProviderEvent
	Delay           time.Duration
	TokensPerSecond int
	MinTokenSize    int
	MaxTokenSize    int
}

// CompleteEntry is a pre-queued complete response.
type CompleteEntry struct {
	Response core.CompleteResponse
	Err      error
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// New creates a fresh Faux Provider with default config from environment variables.
func New() *Provider {
	return &Provider{config: DefaultConfig(), templates: defaultTemplates}
}

// NewWithConfig creates a Faux Provider with an explicit Config.
func NewWithConfig(cfg Config) *Provider {
	return &Provider{config: cfg, templates: defaultTemplates}
}

// ---------------------------------------------------------------------------
// Queue helpers
// ---------------------------------------------------------------------------

// QueueStream appends streaming responses. Call before starting the Loop.
func (p *Provider) QueueStream(responses ...StreamResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.streams = append(p.streams, responses...)
}

// QueueComplete appends complete responses.
func (p *Provider) QueueComplete(entries ...CompleteEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completes = append(p.completes, entries...)
}

// SetFactory sets a dynamic response factory (overrides queue).
func (p *Provider) SetFactory(fn func(context.Context, core.StreamRequest) []StreamResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.factory = fn
}

// CacheStats returns cache reads/writes for a session key.
func (p *Provider) CacheStats(sessionKey string) (reads, writes []int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cs, ok := p.sessions[sessionKey]; ok {
		return cs.cacheReads, cs.cacheWrites
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Config & chaos accessors
// ---------------------------------------------------------------------------

// Config returns a copy of the current Config.
func (p *Provider) Config() Config {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.config
}

// SetConfig replaces the current Config (thread-safe).
func (p *Provider) SetConfig(cfg Config) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = cfg
}

// SetMode is a convenience setter for the chaos mode.
func (p *Provider) SetMode(mode Mode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Mode = mode
}

// ---------------------------------------------------------------------------
// Chaos primitives
// ---------------------------------------------------------------------------

func (p *Provider) applyChaos(mode Mode) error {
	switch mode {
	case ModeDown:
		return fmt.Errorf("faux: provider is down")
	case ModeEvil:
		switch rand.Intn(3) {
		case 0:
			return fmt.Errorf("faux: internal server error")
		case 1:
			return fmt.Errorf("faux: rate limited")
		case 2:
			return fmt.Errorf("faux: timeout")
		}
	case ModeFlaky:
		if rand.Float64() < 0.5 {
			return fmt.Errorf("faux: transient network error")
		}
	}
	return nil
}

func (p *Provider) applyLatency(cfg Config) {
	if cfg.Latency > 0 {
		time.Sleep(cfg.Latency)
		return
	}
	if cfg.LatencyMin > 0 && cfg.LatencyMax > cfg.LatencyMin {
		d := cfg.LatencyMin + time.Duration(rand.Int63n(int64(cfg.LatencyMax-cfg.LatencyMin)))
		time.Sleep(d)
	}
}

func (p *Provider) applyErrors(cfg Config) error {
	if cfg.ErrorRate > 0 && rand.Float64() < cfg.ErrorRate {
		switch rand.Intn(3) {
		case 0:
			return fmt.Errorf("faux: injected 500 error")
		case 1:
			return fmt.Errorf("faux: injected rate limit")
		case 2:
			return fmt.Errorf("faux: injected timeout")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Token budget
// ---------------------------------------------------------------------------

// applyTokenBudget truncates a prompt to stay within the simulated token budget
// (4 chars per token heuristic). Returns the (possibly truncated) prompt.
func applyTokenBudget(prompt string, maxTokens int) string {
	if maxTokens <= 0 {
		return prompt
	}
	maxChars := maxTokens * 4
	if len(prompt) <= maxChars {
		return prompt
	}
	return prompt[:maxChars] + "\n[TRUNCATED — token budget exceeded]"
}

// ---------------------------------------------------------------------------
// Template expansion
// ---------------------------------------------------------------------------

// ExpandTemplate returns a rendered template response for the given name and prompt.
func (p *Provider) ExpandTemplate(name string, prompt string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tmpl, ok := p.templates[name]
	if !ok {
		return "", fmt.Errorf("faux: unknown template %q", name)
	}
	return strings.ReplaceAll(tmpl.Pattern, "{{.Prompt}}", prompt), nil
}

// TemplateStreamResponse builds a StreamResponse from a named template.
func (p *Provider) TemplateStreamResponse(name string, prompt string) (StreamResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tmpl, ok := p.templates[name]
	if !ok {
		return StreamResponse{}, fmt.Errorf("faux: unknown template %q", name)
	}
	text := strings.ReplaceAll(tmpl.Pattern, "{{.Prompt}}", prompt)
	msgID := fmt.Sprintf("faux-tmpl-%d", time.Now().UnixNano())
	return StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: msgID},
			{Type: core.ProvContentDelta, MessageID: msgID, ContentDelta: text},
			{Type: core.ProvMessageEnd, MessageID: msgID},
		},
		TokensPerSecond: tmpl.TokensPerSecond,
	}, nil
}

// ---------------------------------------------------------------------------
// Record-replay
// ---------------------------------------------------------------------------

// Record saves a sequence of provider events to a JSON file.
func (p *Provider) Record(filename string, events []core.ProviderEvent) error {
	var recorded []RecordedEvent
	now := time.Now()
	for _, ev := range events {
		recorded = append(recorded, RecordedEvent{Time: now, Event: ev})
		now = now.Add(50 * time.Millisecond)
	}
	data, err := json.MarshalIndent(recorded, "", "  ")
	if err != nil {
		return fmt.Errorf("faux: marshal record: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

// Replay loads recorded events from a JSON file.
func (p *Provider) Replay(filename string) ([]core.ProviderEvent, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("faux: read replay file: %w", err)
	}
	var recorded []RecordedEvent
	if err := json.Unmarshal(data, &recorded); err != nil {
		return nil, fmt.Errorf("faux: unmarshal replay: %w", err)
	}
	events := make([]core.ProviderEvent, len(recorded))
	for i, r := range recorded {
		events[i] = r.Event
	}
	return events, nil
}

// ---------------------------------------------------------------------------
// Stream & Complete (core.Provider interface)
// ---------------------------------------------------------------------------

// Stream dequeues and replays the next queued streaming response.
// Chaos checks run BEFORE queue/factory processing.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	p.mu.Lock()
	cfg := p.config
	p.mu.Unlock()

	// 1. Chaos mode error
	if err := p.applyChaos(cfg.Mode); err != nil {
		return nil, err
	}
	// 2. Latency simulation
	p.applyLatency(cfg)
	// 3. Error injection
	if err := p.applyErrors(cfg); err != nil {
		return nil, err
	}

	// Extract prompt for token budget and template rendering
	promptText := ""
	for _, msg := range req.Messages {
		if msg.Role == core.RoleUser {
			promptText += msg.Content
		}
	}

	p.mu.Lock()
	// 4. Replay mode: load from file
	if cfg.ReplayFile != "" {
		events, err := p.Replay(cfg.ReplayFile)
		if err != nil {
			p.mu.Unlock()
			return nil, err
		}
		resp := StreamResponse{Events: events}
		p.mu.Unlock()
		ch := make(chan core.ProviderEvent, len(resp.Events)+1)
		go p.replayEvents(ctx, ch, resp)
		return ch, nil
	}

	// 5. Factory path
	if p.factory != nil {
		responses := p.factory(ctx, req)
		p.mu.Unlock()
		if len(responses) == 0 {
			return nil, fmt.Errorf("faux: factory returned no responses")
		}
		resp := responses[0]
		ch := make(chan core.ProviderEvent, len(resp.Events)+1)
		go p.replayEvents(ctx, ch, resp)
		return ch, nil
	}

	// 6. Queue-based path
	if p.streamIdx >= len(p.streams) {
		p.mu.Unlock()
		return nil, fmt.Errorf("faux: no queued streaming responses (have %d, used %d)", len(p.streams), p.streamIdx)
	}
	resp := p.streams[p.streamIdx]
	p.streamIdx++

	// Token budget simulation: truncate prompt in ContentDelta events
	if cfg.MaxTokens > 0 && len(promptText) > cfg.MaxTokens*4 {
		for i := range resp.Events {
			if resp.Events[i].Type == core.ProvContentDelta && len(resp.Events[i].ContentDelta) > cfg.MaxTokens*4 {
				resp.Events[i].ContentDelta = applyTokenBudget(resp.Events[i].ContentDelta, cfg.MaxTokens)
			}
		}
	}

	// Simulate prompt cache
	if p.sessions == nil {
		p.sessions = make(map[string]*cacheState)
	}
	sessID := fmt.Sprintf("%v", req.Model.Name)
	cs := p.sessions[sessID]
	if cs == nil {
		cs = &cacheState{}
		p.sessions[sessID] = cs
	}
	cs.cacheReads = append(cs.cacheReads, cs.commonPrefix)
	newPrefix := len(req.Messages)
	cs.cacheWrites = append(cs.cacheWrites, newPrefix-cs.commonPrefix)
	cs.commonPrefix = newPrefix
	p.mu.Unlock()

	ch := make(chan core.ProviderEvent, len(resp.Events)+1)
	go p.replayEvents(ctx, ch, resp)

	// Record mode
	if cfg.RecordFile != "" {
		go func() {
			var events []core.ProviderEvent
			for ev := range ch {
				events = append(events, ev)
			}
			_ = p.Record(cfg.RecordFile, events)
		}()
		return ch, nil
	}

	return ch, nil
}

// Complete dequeues and returns the next queued complete response.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	p.mu.Lock()
	cfg := p.config
	p.mu.Unlock()

	// Chaos checks
	if err := p.applyChaos(cfg.Mode); err != nil {
		return core.CompleteResponse{}, err
	}
	p.applyLatency(cfg)
	if err := p.applyErrors(cfg); err != nil {
		return core.CompleteResponse{}, err
	}

	p.mu.Lock()
	if p.completeIdx >= len(p.completes) {
		p.mu.Unlock()
		return core.CompleteResponse{}, fmt.Errorf("faux: no queued complete responses")
	}
	entry := p.completes[p.completeIdx]
	p.completeIdx++
	p.mu.Unlock()
	return entry.Response, entry.Err
}

// ---------------------------------------------------------------------------
// replayEvents (unchanged from original)
// ---------------------------------------------------------------------------

func (p *Provider) replayEvents(ctx context.Context, ch chan<- core.ProviderEvent, resp StreamResponse) {
	defer close(ch)
	for _, ev := range resp.Events {
		if ev.Type == core.ProvContentDelta && resp.TokensPerSecond > 0 && ev.ContentDelta != "" {
			chunks := splitIntoChunks(ev.ContentDelta, resp.MinTokenSize, resp.MaxTokenSize)
			delayPerChunk := time.Second / time.Duration(resp.TokensPerSecond)
			for _, chunk := range chunks {
				select {
				case <-ctx.Done():
					ch <- core.ProviderEvent{
						Type:         core.ProvContentDelta,
						ContentDelta: "\n[aborted]",
					}
					return
				case ch <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    ev.MessageID,
					ContentDelta: chunk,
				}:
				}
				if delayPerChunk > 0 {
					select {
					case <-ctx.Done():
						ch <- core.ProviderEvent{
							Type:         core.ProvContentDelta,
							ContentDelta: "\n[aborted]",
						}
						return
					case <-time.After(delayPerChunk):
					}
				}
			}
		} else {
			select {
			case <-ctx.Done():
				ch <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					ContentDelta: "\n[aborted]",
				}
				return
			case ch <- ev:
			}
			if resp.Delay > 0 {
				select {
				case <-ctx.Done():
					ch <- core.ProviderEvent{
						Type:         core.ProvContentDelta,
						ContentDelta: "\n[aborted]",
					}
					return
				case <-time.After(resp.Delay):
				}
			}
		}
	}
}

func splitIntoChunks(text string, minSize, maxSize int) []string {
	if minSize <= 0 {
		minSize = 3
	}
	if maxSize <= 0 {
		maxSize = 10
	}
	if len(text) <= maxSize {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		size := minSize + rand.Intn(maxSize-minSize+1)
		if size > len(text) {
			size = len(text)
		}
		chunks = append(chunks, text[:size])
		text = text[size:]
	}
	return chunks
}