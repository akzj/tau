package core

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// SpanID is a unique span identifier.
type SpanID uint64

var nextSpanID atomic.Uint64

// Span represents a single operation trace.
type Span struct {
	ID       SpanID            `json:"id"`
	ParentID SpanID            `json:"parent_id,omitempty"`
	Name     string            `json:"name"`
	Tags     map[string]string `json:"tags,omitempty"`
	Status   string            `json:"status"` // "ok", "error"
	Start    time.Time         `json:"start"`
	End      time.Time         `json:"end,omitempty"`
	Duration string            `json:"duration,omitempty"`
}

// Tracer manages spans and emits trace events.
type Tracer struct {
	mu      sync.Mutex
	spans   []Span
	enabled bool
}

var globalTracer = &Tracer{enabled: os.Getenv("TAU_TRACE") == "1"}

// StartSpan begins a new span with the given name. Returns a function to end the span.
func StartSpan(name string, tags map[string]string) func(status string) {
	if !globalTracer.enabled {
		return func(string) {}
	}

	id := SpanID(nextSpanID.Add(1))
	span := Span{ID: id, Name: name, Tags: tags, Start: time.Now(), Status: "ok"}

	globalTracer.mu.Lock()
	globalTracer.spans = append(globalTracer.spans, span)
	idx := len(globalTracer.spans) - 1
	globalTracer.mu.Unlock()

	return func(status string) {
		globalTracer.mu.Lock()
		s := &globalTracer.spans[idx]
		s.End = time.Now()
		s.Duration = s.End.Sub(s.Start).String()
		s.Status = status
		data, _ := json.Marshal(s)
		fmt.Fprintf(os.Stderr, "%s\n", data)
		globalTracer.mu.Unlock()
	}
}

// TraceEnabled returns true if TAU_TRACE=1.
func TraceEnabled() bool { return globalTracer.enabled }

// Flush returns a copy of all spans.
func (t *Tracer) Flush() []Span {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]Span, len(t.spans))
	copy(result, t.spans)
	return result
}

// GetTracer returns the global tracer.
func GetTracer() *Tracer { return globalTracer }