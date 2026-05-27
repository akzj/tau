package core

import (
	"context"
	"fmt"
	"strings"
)

// PlanStrategy wraps the agent loop with plan tracking.
// Injects plan progress into system prompt without touching loop.go.
type PlanStrategy struct {
	inner AgentStrategy
	plan  *Plan
}

// NewPlanStrategy wraps an inner strategy with plan tracking.
func NewPlanStrategy(inner AgentStrategy, plan *Plan) *PlanStrategy {
	return &PlanStrategy{inner: inner, plan: plan}
}

// Name returns "plan-" + inner strategy name.
func (s *PlanStrategy) Name() string { return "plan-" + s.inner.Name() }

// Decide updates plan status from observations, gets next todo, delegates to inner.
func (s *PlanStrategy) Decide(ctx context.Context, state AgentState) (Decision, error) {
	// Update plan status from observations
	s.updateFromObservations(state.Observations)

	// Get next todo
	next := s.plan.GetNextTodo()
	if next != nil {
		s.plan.MarkStatus(next.ID, PlanInProgress)
		return Decision{
			Action: "continue",
			Reason: fmt.Sprintf("[plan] next: %s (%s)", next.Title, s.progressString()),
		}, nil
	}

	// Check if complete
	if s.plan.IsComplete() {
		return Decision{
			Action: "stop",
			Reason: fmt.Sprintf("[plan] ✅ complete (%s)", s.progressString()),
		}, nil
	}

	return s.inner.Decide(ctx, state)
}

func (s *PlanStrategy) updateFromObservations(obs []string) {
	for _, o := range obs {
		if containsCompletion(o) {
			// Mark current in-progress as done
			for _, c := range s.plan.Root.Children {
				if c.Status == PlanInProgress {
					s.plan.MarkStatus(c.ID, PlanDone)
					break
				}
			}
		}
	}
}

func (s *PlanStrategy) progressString() string {
	done, total := s.plan.Progress()
	return fmt.Sprintf("%d/%d done", done, total)
}

func containsCompletion(s string) bool {
	return strings.Contains(s, "completed") || strings.Contains(s, "done") || strings.Contains(s, "finished")
}

func init() {
	RegisterStrategy("plan", func() AgentStrategy {
		plan := NewPlan("default")
		return NewPlanStrategy(NewReActStrategy(), plan)
	})
}
