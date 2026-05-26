//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

func main() {
	ctx := context.Background()
	prov := faux.New()

	// Queue: assistant says "hello world" as two deltas
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "hello "},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "world"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     prov,
		DefaultModel: core.ModelSpec{Name: "faux", API: core.WireOpenAICompletions},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Cancel()

	loop := core.NewLoop()
	run, err := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "prompt: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== faux demo ===")
	for ev := range run.Events() {
		switch e := ev.(type) {
		case core.MessageDelta:
			fmt.Print(e.ContentDelta)
		case core.MessageEnd:
			fmt.Println()
		case core.TurnEnd:
			fmt.Printf("[turn end: %s]\n", e.Reason)
		}
	}
	<-run.Done()
	fmt.Println("=== PASS ===")
}