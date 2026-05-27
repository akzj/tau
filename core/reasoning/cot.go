package reasoning

import (
	"context"
	"fmt"
	"strings"

	"github.com/akzj/tau/core"
)

// CoTStrategy implements Chain-of-Thought with structured output.
// Agent writes reasoning step-by-step, then produces a conclusion.
type CoTStrategy struct {
	inner core.AgentStrategy
	steps int
}

// NewCoTStrategy creates a Chain-of-Thought strategy wrapping the given inner strategy.
// steps configures the number of reasoning steps (default 5 if <= 0).
func NewCoTStrategy(inner core.AgentStrategy, steps int) *CoTStrategy {
	if steps <= 0 {
		steps = 5
	}
	return &CoTStrategy{inner: inner, steps: steps}
}

// Name returns "cot".
func (s *CoTStrategy) Name() string { return "cot" }

// Decide builds a structured step-by-step reasoning prompt from the last user message.
func (s *CoTStrategy) Decide(ctx context.Context, state core.AgentState) (core.Decision, error) {
	var b strings.Builder
	b.WriteString("Let's think this through step by step.\n\n")

	if len(state.Messages) > 0 {
		lastMsg := state.Messages[len(state.Messages)-1]
		b.WriteString(fmt.Sprintf("Question: %s\n\n", lastMsg.Content))
	}

	for i := 1; i <= s.steps; i++ {
		b.WriteString(fmt.Sprintf("Step %d: [reason about this aspect]\n", i))
	}
	b.WriteString("\nConclusion: [final answer]\n")

	return core.Decision{
		Action: "system_prompt",
		Reason: b.String(),
	}, nil
}

func init() {
	core.RegisterStrategy("cot", func() core.AgentStrategy {
		return NewCoTStrategy(core.GetStrategy("react"), 5)
	})
}
