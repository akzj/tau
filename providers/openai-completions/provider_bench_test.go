package openai_completions

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

func BenchmarkSSEParse(b *testing.B) {
	var chunks []string
	for i := 0; i < 100; i++ {
		chunks = append(chunks, `data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"hello "},"index":0}]}`)
	}
	chunks = append(chunks, `data: [DONE]`)
	data := strings.Join(chunks, "\n") + "\n"

	prov := &OpenAICompletionsProvider{}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := io.NopCloser(strings.NewReader(data))
		events := make(chan core.ProviderEvent, 128)
		go func() {
			for range events {
			}
		}()
		prov.parseSSE(ctx, body, events)
	}
}