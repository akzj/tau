package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
	"github.com/akzj/tau/pkg/tui"
	"github.com/akzj/tau/pkg/webui"
	"github.com/akzj/tau/pkg/persist"
	"github.com/akzj/tau/providers/anthropic-messages"
	"github.com/akzj/tau/providers/openai-completions"
)

func main() {
	// Flags
	providerName := flag.String("provider", "openai", "Provider (openai or anthropic)")
	model := flag.String("model", "gpt-5.4", "Model name")
	workspace := flag.String("workspace", "", "Workspace root (default: cwd)")
	verbose := flag.Bool("verbose", false, "Verbose output")
	noTools := flag.Bool("no-tools", false, "Disable all tools")
	maxTurns := flag.Int("max-turns", 10, "Max agent turns")
	tuiMode := flag.Bool("tui", false, "Launch Terminal UI")
	webuiMode := flag.Bool("webui", false, "Launch Web UI")
	addr := flag.String("addr", ":8080", "Web UI listen address")
	resumeID := flag.String("resume", "", "Resume a saved session by ID")
	steerMsg := flag.String("steer", "", "Inject a steer instruction (for use with --resume)")
	listSessions := flag.Bool("list-sessions", false, "List saved sessions")
	flag.Parse()

	// Resolve workspace
	wsRoot := *workspace
	if wsRoot == "" {
		var err error
		wsRoot, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "getcwd: %v\n", err)
			os.Exit(1)
		}
	}

	// Get prompt from args or stdin
	var prompt string
	if flag.NArg() > 0 {
		prompt = strings.Join(flag.Args(), " ")
	} else {
		data, _ := io.ReadAll(os.Stdin)
		prompt = strings.TrimSpace(string(data))
	}
	if prompt == "" && !*tuiMode && !*webuiMode && !*listSessions {
		fmt.Fprintf(os.Stderr, "Usage: tau [flags] <prompt>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// --list-sessions: print saved sessions and exit
	if *listSessions {
		sessions, err := persist.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "list: %v\n", err)
			os.Exit(1)
		}
		if len(sessions) == 0 {
			fmt.Println("No saved sessions.")
			return
		}
		fmt.Printf("%-20s %-12s %s\n", "ID", "MSGS", "FIRST MESSAGE")
		for _, s := range sessions {
			fmt.Printf("%-20s %-12d %s\n", s.ID, s.MsgCount, s.FirstMsg)
		}
		return
	}

	ctx := context.Background()

	// Create provider
	var prov core.Provider
	var api core.WireAPI
	switch *providerName {
	case "anthropic":
		p, err := anthropic_messages.NewAnthropicMessagesProvider()
		if err != nil {
			fmt.Fprintf(os.Stderr, "provider: %v\n", err)
			os.Exit(1)
		}
		prov = p
		api = core.WireAnthropicMessages
		if *model == "gpt-5.4" {
			*model = "claude-sonnet-4-6"
		}
	default:
		p, err := openai_completions.NewOpenAICompletionsProvider()
		if err != nil {
			fmt.Fprintf(os.Stderr, "provider: %v\n", err)
			os.Exit(1)
		}
		prov = p
		api = core.WireOpenAICompletions
	}

	// System prompt
	systemPrompt := func(s *core.Session) (string, error) {
		return fmt.Sprintf("You are tau, a coding agent. Workspace: %s. Use tools to read, write, and execute code. Always verify your changes.", wsRoot), nil
	}

	// Create session
	sess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
		WorkspaceRoot: wsRoot,
		SystemPrompt:  systemPrompt,
		Provider:      prov,
		DefaultModel: core.ModelSpec{
			Name: *model,
			API:  api,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Cancel()

	// Resume: load saved transcript history
	if *resumeID != "" {
		msgs, err := persist.Load(*resumeID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resume: %v\n", err)
			os.Exit(1)
		}
		sess.Session.Transcript.Append(msgs...)
		if *verbose {
			fmt.Fprintf(os.Stderr, "[resumed %s] (%d messages)\n", *resumeID, len(msgs))
		}
	}

	// Inject steer instruction if provided with --resume
	if *steerMsg != "" {
		sess.Session.Steer(*steerMsg)
		if *verbose {
			fmt.Fprintf(os.Stderr, "[steer] %s\n", *steerMsg)
		}
	}

	// Disable tools if requested
	if *noTools {
		sess.Tools.SetActive(nil)
	}

	loop := core.NewLoop()

	// WebUI mode: launch HTTP+WebSocket server, skip batch loop.
	if *webuiMode {
		srv := webui.NewServer(*addr, wsRoot, *model, prov)
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "webui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// TUI mode: launch Bubble Tea, skip batch loop.
	if *tuiMode {
		tm := tui.NewModel(sess, loop, prompt)
		p := tea.NewProgram(tm, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "tui: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Main loop
	currentPrompt := prompt

	for turn := 0; turn < *maxTurns; turn++ {
		if *verbose {
			fmt.Fprintf(os.Stderr, "[turn %d]\n", turn+1)
		}

		var run *core.Run
		if turn == 0 {
			run, err = loop.Prompt(ctx, sess.Session, core.UserInput{Text: currentPrompt})
		} else {
			run, err = loop.Continue(ctx, sess.Session)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Consume events
		timeout := time.After(300 * time.Second)
		turnDone := false
		for !turnDone {
			select {
			case ev, ok := <-run.Events():
				if !ok {
					turnDone = true
					break
				}
				printEvent(ev, *verbose)
			case <-timeout:
				fmt.Fprintln(os.Stderr, "[TIMEOUT]")
				run.Cancel()
				turnDone = true
			case <-ctx.Done():
				turnDone = true
			}
		}
		<-run.Done()
	}

	// Save session on exit
	sessionID := persist.NewID()
	msgs := sess.Transcript.Messages()
	if err := persist.Save(sessionID, msgs); err != nil {
		fmt.Fprintf(os.Stderr, "save: %v\n", err)
	} else if *verbose {
		fmt.Fprintf(os.Stderr, "[saved %s] (%d messages)\n", sessionID, len(msgs))
	}

	fmt.Println()
}

func printEvent(ev core.AgentEvent, verbose bool) {
	switch e := ev.(type) {
	case core.TurnStart:
		if verbose {
			fmt.Fprintf(os.Stderr, "[turn start]\n")
		}
	case core.MessageStart:
		// no print
	case core.MessageDelta:
		fmt.Print(e.ContentDelta)
	case core.MessageEnd:
		fmt.Println()
	case core.ToolCallStart:
		if verbose {
			fmt.Fprintf(os.Stderr, "[tool: %s]\n", e.ToolName)
		}
	case core.ToolCallEnd:
		if verbose {
			for _, c := range e.Result.Content {
				if c.Type == "text" && len(c.Text) > 200 {
					fmt.Fprintf(os.Stderr, "[tool result] %s...\n", c.Text[:200])
				} else if c.Type == "text" {
					fmt.Fprintf(os.Stderr, "[tool result] %s\n", c.Text)
				}
			}
		}
	case core.TurnEnd:
		if verbose {
			fmt.Fprintf(os.Stderr, "[turn end] %s\n", e.Reason)
		}
	case core.ErrorEvent:
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", e.Err)
	}
}
