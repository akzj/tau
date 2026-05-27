package core

import "context"

// ReflectionStrategy wraps an existing strategy with output reflection.
// After the inner strategy's Decide returns, it reviews the last assistant
// output and, if issues are found, injects a correction prompt.
type ReflectionStrategy struct {
	inner  AgentStrategy
	engine *ReflectionEngine
}

// NewReflectionStrategy creates a reflection-wrapped strategy.
func NewReflectionStrategy(inner AgentStrategy, maxRounds int) *ReflectionStrategy {
	return &ReflectionStrategy{
		inner:  inner,
		engine: NewReflectionEngine(maxRounds),
	}
}

func (s *ReflectionStrategy) Name() string { return "reflect-" + s.inner.Name() }

func (s *ReflectionStrategy) Decide(ctx context.Context, state AgentState) (Decision, error) {
	// First, get the inner strategy's decision
	decision, err := s.inner.Decide(ctx, state)
	if err != nil {
		return decision, err
	}

	// If the decision is "stop" with content, review it
	if decision.Action == "stop" && decision.Reason != "" {
		result := s.engine.Review(decision.Reason)
		if !result.Passed {
			correctionPrompt := s.engine.BuildCorrectionPrompt(decision.Reason, result.Issues)
			// Inject correction as new observation — continue the loop
			return Decision{
				Action: "continue",
				Reason: correctionPrompt,
			}, nil
		}
	}

	return decision, nil
}

func init() {
	RegisterStrategy("reflect", func() AgentStrategy {
		return NewReflectionStrategy(NewReActStrategy(), 2)
	})
}
