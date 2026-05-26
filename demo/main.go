package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/akzj/tau/core"
	openai_completions "github.com/akzj/tau/providers/openai-completions"
	"github.com/akzj/tau/toolspec"
)

func main() {
	ctx := context.Background()

	// 1. Create provider
	prov, err := openai_completions.NewOpenAICompletionsProvider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider: %v\n", err)
		os.Exit(1)
	}

	// 2. Create session
	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider: prov,
		DefaultModel: core.ModelSpec{
			Name: "gpt-5.4",
			API:  core.WireOpenAICompletions,
		},
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

	// Initialize memory system for demo
	memDir, _ := os.MkdirTemp("", "tau-memory-demo-*")
	defer os.RemoveAll(memDir)
	sess.Memory = core.NewMemorySystem(memDir, memDir)

	// 4. Multi-turn: Prompt → Continue
	fmt.Println("=== tau demo: multi-turn echo ===")
	fmt.Println()

	loop := core.NewLoop()

	// --- Turn 1: user asks to use echo tool ---
	fmt.Println("--- Turn 1: Prompt ---")
	run1, err := loop.Prompt(ctx, sess, core.UserInput{Text: "Please use the echo tool with message 'hello tau'"})
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

	// Show memory activity
	if sess.Memory != nil {
		fmt.Fprintf(os.Stderr, "\n[memory] Working Memory Summary:\n")
		summary := sess.Memory.Working.Summarize()
		for _, line := range strings.Split(summary, "\n") {
			fmt.Fprintf(os.Stderr, "[memory]   %s\n", line)
		}
		fmt.Fprintf(os.Stderr, "[memory] Stats — Working: %d obs, Episodic: %d episodes\n",
			sess.Memory.Working.Len(), sess.Memory.Episodic.Len())
	}

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

	// Show memory activity after turn 2
	if sess.Memory != nil {
		fmt.Fprintf(os.Stderr, "\n[memory] Working Memory Summary:\n")
		summary := sess.Memory.Working.Summarize()
		for _, line := range strings.Split(summary, "\n") {
			fmt.Fprintf(os.Stderr, "[memory]   %s\n", line)
		}
		fmt.Fprintf(os.Stderr, "[memory] Stats — Working: %d obs, Episodic: %d episodes\n",
			sess.Memory.Working.Len(), sess.Memory.Episodic.Len())
	}

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
