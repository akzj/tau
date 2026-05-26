//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/akzj/tau/core"
	anthropic_messages "github.com/akzj/tau/providers/anthropic-messages"
	"github.com/akzj/tau/toolspec"
)

func main() {
	ctx := context.Background()

	// 1. Create provider
	prov, err := anthropic_messages.NewAnthropicMessagesProvider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider: %v\n", err)
		os.Exit(1)
	}

	// 2. Create session
	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider: prov,
		SystemPrompt: func(s *core.Session) (string, error) {
			return "You are a helpful assistant. Use tools when needed.", nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Cancel()

	// 3. Register echo tool
	sess.Tools.Register(core.Tool{
		Name:        "echo",
		Description: "Echo back the input message.",
		Schema:      toolspec.EchoToolSchema{},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			// Handle nil params
			if params == nil {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ECHO: (nil params)"}}}, nil
			}
			// Loop passes json.RawMessage as params
			var raw json.RawMessage
			switch v := params.(type) {
			case json.RawMessage:
				raw = v
			case map[string]any:
				b, _ := json.Marshal(v)
				raw = b
			default:
				return core.ToolResult{}, fmt.Errorf("unexpected params type: %T", params)
			}

			var args struct{ Msg string }
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.ToolResult{}, fmt.Errorf("unmarshal echo args: %w", err)
			}
			if args.Msg == "" {
				args.Msg = "(empty)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("ECHO: %s", args.Msg)}},
			}, nil
		},
	})
	sess.Tools.SetActive([]string{"echo"})
	sess.Providers.RegisterProvider("default", prov)

	// 4. Multi-turn: Prompt → Continue
	fmt.Println("=== tau demo: Anthropic multi-turn echo ===")
	fmt.Println()

	loop := core.NewLoop()

	// --- Turn 1: user asks to use echo tool ---
	fmt.Println("--- Turn 1: Prompt ---")
	run1, err := loop.Prompt(ctx, sess, core.UserInput{Text: "Please use the echo tool with message 'hello anthropic'"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "prompt: %v\n", err)
		os.Exit(1)
	}

	timeout1 := time.After(60 * time.Second)
	turn1Done := false
	for !turn1Done {
		select {
		case ev, ok := <-run1.Events():
			if !ok {
				turn1Done = true
			} else {
				printEvent(ev)
			}
		case <-timeout1:
			fmt.Println("⏰ Timeout")
			run1.Cancel()
			turn1Done = true
		case <-ctx.Done():
			turn1Done = true
		}
	}
	<-run1.Done()

	// --- Turn 2: Continue (send tool results back to LLM) ---
	fmt.Println()
	fmt.Println("--- Turn 2: Continue ---")
	run2, err := loop.Continue(ctx, sess)
	if err != nil {
		fmt.Fprintf(os.Stderr, "continue: %v\n", err)
		os.Exit(1)
	}

	timeout2 := time.After(60 * time.Second)
	turn2Done := false
	for !turn2Done {
		select {
		case ev, ok := <-run2.Events():
			if !ok {
				turn2Done = true
			} else {
				printEvent(ev)
			}
		case <-timeout2:
			fmt.Println("⏰ Timeout")
			run2.Cancel()
			turn2Done = true
		case <-ctx.Done():
			turn2Done = true
		}
	}
	<-run2.Done()

	fmt.Println()
	fmt.Println("=== done ===")
}

func printEvent(ev core.AgentEvent) {
	switch e := ev.(type) {
	case core.TurnStart:
		fmt.Printf("[turn start] id=%s\n", e.TurnID)
	case core.MessageStart:
		fmt.Printf("[message start] id=%s role=%s\n", e.MessageID, e.Role)
	case core.MessageDelta:
		fmt.Print(e.ContentDelta)
	case core.MessageEnd:
		fmt.Printf("\n[message end] id=%s\n", e.MessageID)
	case core.ToolCallStart:
		fmt.Printf("[tool call start] call=%s tool=%s\n", e.CallID, e.ToolName)
	case core.ToolCallUpdate:
		fmt.Printf("[tool call update] call=%s delta=%s\n", e.CallID, e.Partial.ContentDelta)
	case core.ToolCallEnd:
		fmt.Printf("[tool call end] call=%s result=%v\n", e.CallID, e.Result.Content)
	case core.TurnEnd:
		fmt.Printf("[turn end] id=%s reason=%s\n", e.TurnID, e.Reason)
	case core.ErrorEvent:
		fmt.Printf("[ERROR] code=%s err=%v\n", e.Code, e.Err)
	default:
		fmt.Printf("[unknown event] %T\n", ev)
	}
}