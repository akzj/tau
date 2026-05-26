package core

import "context"

// ReActStrategy implements the standard Reason+Act loop.
type ReActStrategy struct{}

// NewReActStrategy creates a ReAct strategy.
func NewReActStrategy() *ReActStrategy { return &ReActStrategy{} }

// Name returns "react".
func (s *ReActStrategy) Name() string { return "react" }

// Decide determines the next action using ReAct: observe→think→act.
func (s *ReActStrategy) Decide(ctx context.Context, state AgentState) (Decision, error) {
	if state.LastError != nil {
		return Decision{Action: "continue", Reason: "retry after error"}, nil
	}
	if state.TurnCount >= state.MaxTurns {
		return Decision{Action: "stop", Reason: "max turns reached"}, nil
	}
	return Decision{Action: "continue", Reason: "ReAct: observe→think→act"}, nil
}

func init() {
	RegisterStrategy("react", func() AgentStrategy { return NewReActStrategy() })
}
