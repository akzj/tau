package core

import "context"

// CoTStrategy implements Chain-of-Thought reasoning.
type CoTStrategy struct {
	reasoningDone bool
}

// NewCoTStrategy creates a Chain-of-Thought strategy.
func NewCoTStrategy() *CoTStrategy { return &CoTStrategy{} }

// Name returns "cot".
func (s *CoTStrategy) Name() string { return "cot" }

// Decide first reasons, then acts.
func (s *CoTStrategy) Decide(ctx context.Context, state AgentState) (Decision, error) {
	if state.TurnCount >= state.MaxTurns {
		return Decision{Action: "stop", Reason: "max turns"}, nil
	}

	if !s.reasoningDone {
		s.reasoningDone = true
		return Decision{Action: "continue", Reason: "CoT: think step by step before acting"}, nil
	}

	return Decision{Action: "continue", Reason: "CoT: execute action based on reasoning"}, nil
}

func init() {
	RegisterStrategy("cot", func() AgentStrategy { return NewCoTStrategy() })
}
