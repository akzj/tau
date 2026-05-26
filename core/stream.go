package core

import (
	"context"
	"fmt"
)

// StreamEventType classifies a streaming event.
type StreamEventType string

const (
	// StreamDelta carries a text chunk.
	StreamDelta StreamEventType = "delta"
	// StreamDone signals the stream has ended.
	StreamDone StreamEventType = "done"
	// StreamError carries an error that occurred during streaming.
	StreamError StreamEventType = "error"
)

// StreamEvent is pushed to UIs during streaming.
type StreamEvent struct {
	Type    StreamEventType
	Content string // text delta
	Tool    string // tool call name (if applicable)
	Err     error  // on StreamError
}

// ErrNotSupported is returned when a provider doesn't support streaming.
var ErrNotSupported = fmt.Errorf("streaming not supported by this provider")

// StreamCall adapts a Provider.Stream call to emit StreamEvents for UI consumption.
// Falls back gracefully to Complete if the provider doesn't support streaming.
func StreamCall(ctx context.Context, prov Provider, req StreamRequest) (<-chan StreamEvent, error) {
	ch, err := prov.Stream(ctx, req)
	if err != nil {
		if err == ErrNotSupported {
			out := make(chan StreamEvent, 1)
			go func() {
				defer close(out)
				resp, err := prov.Complete(ctx, CompleteRequest{
					Model:    req.Model,
					Messages: req.Messages,
					Options:  req.Options,
				})
				if err != nil {
					out <- StreamEvent{Type: StreamError, Err: err}
					return
				}
				out <- StreamEvent{Type: StreamDelta, Content: resp.Content}
				out <- StreamEvent{Type: StreamDone}
			}()
			return out, nil
		}
		return nil, fmt.Errorf("stream call: %w", err)
	}

	out := make(chan StreamEvent, 64)
	go func() {
		defer close(out)
		for ev := range ch {
			switch ev.Type {
			case ProvContentDelta:
				out <- StreamEvent{Type: StreamDelta, Content: ev.ContentDelta}
			case ProvToolCallStart:
				out <- StreamEvent{Type: StreamDelta, Tool: ev.ToolName}
			case ProvError:
				out <- StreamEvent{Type: StreamError, Err: ev.Err}
			}
		}
		out <- StreamEvent{Type: StreamDone}
	}()
	return out, nil
}
