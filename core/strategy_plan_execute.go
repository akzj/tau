package core

import (
	"context"
	"fmt"
	"strings"
)

// PlanExecuteStrategy generates a plan, executes step by step, replans on error.
type PlanExecuteStrategy struct {
	steps  []string
	cursor int
}

// NewPlanExecuteStrategy creates a Plan-Execute strategy.
func NewPlanExecuteStrategy() *PlanExecuteStrategy {
	return &PlanExecuteStrategy{}
}

// Name returns "plan-execute".
func (s *PlanExecuteStrategy) Name() string { return "plan-execute" }

// Decide generates a plan on first call, advances steps, replans on error.
func (s *PlanExecuteStrategy) Decide(ctx context.Context, state AgentState) (Decision, error) {
	if len(s.steps) == 0 {
		s.steps = s.generatePlan(state)
		s.cursor = 0
		if len(s.steps) == 0 {
			return Decision{Action: "stop", Reason: "no plan generated"}, nil
		}
		return Decision{
			Action: fmt.Sprintf("tool:%s", s.steps[0]),
			Reason: fmt.Sprintf("Plan step 1/%d: %s", len(s.steps), s.steps[0]),
		}, nil
	}

	if state.LastError != nil {
		s.steps = s.generatePlan(state)
		s.cursor = 0
		if len(s.steps) == 0 {
			return Decision{Action: "stop", Reason: "replan failed"}, nil
		}
		return Decision{
			Action: fmt.Sprintf("tool:%s", s.steps[0]),
			Reason: fmt.Sprintf("Replan after error: step 1/%d: %s", len(s.steps), s.steps[0]),
		}, nil
	}

	s.cursor++
	if s.cursor >= len(s.steps) {
		return Decision{Action: "stop", Reason: "plan completed"}, nil
	}
	return Decision{
		Action: fmt.Sprintf("tool:%s", s.steps[s.cursor]),
		Reason: fmt.Sprintf("Plan step %d/%d: %s", s.cursor+1, len(s.steps), s.steps[s.cursor]),
	}, nil
}

func (s *PlanExecuteStrategy) generatePlan(state AgentState) []string {
	if len(state.Messages) == 0 {
		return nil
	}
	lastMsg := state.Messages[len(state.Messages)-1]
	content := strings.ToLower(lastMsg.Content)

	var plan []string
	if strings.Contains(content, "read") || strings.Contains(content, "file") {
		plan = append(plan, "read", "analyze", "respond")
	} else if strings.Contains(content, "build") || strings.Contains(content, "test") {
		plan = append(plan, "read", "bash", "respond")
	} else {
		plan = append(plan, "think", "respond")
	}
	return plan
}

func init() {
	RegisterStrategy("plan-execute", func() AgentStrategy { return NewPlanExecuteStrategy() })
}
