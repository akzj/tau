package reasoning

import (
	"context"
	"fmt"
	"strings"

	"github.com/akzj/tau/core"
)

// DecomposeStrategy splits a complex task into parallel sub-tasks.
// Uses keyword-based decomposition for the mock-testable path.
type DecomposeStrategy struct {
	inner       core.AgentStrategy
	maxSubTasks int
}

// NewDecomposeStrategy creates a Decomposition strategy (max 5 sub-tasks).
func NewDecomposeStrategy(inner core.AgentStrategy) *DecomposeStrategy {
	return &DecomposeStrategy{inner: inner, maxSubTasks: 5}
}

// Name returns "decompose".
func (s *DecomposeStrategy) Name() string { return "decompose" }

// Decide decomposes the last user message into sub-tasks and returns a delegate decision.
func (s *DecomposeStrategy) Decide(ctx context.Context, state core.AgentState) (core.Decision, error) {
	if len(state.Messages) == 0 {
		return core.Decision{Action: "stop", Reason: "no input"}, nil
	}

	lastMsg := state.Messages[len(state.Messages)-1]
	subtasks := s.decompose(lastMsg.Content)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[Decompose] split into %d sub-tasks:\n", len(subtasks)))
	for i, st := range subtasks {
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, st))
	}

	return core.Decision{
		Action: "delegate",
		Reason: b.String(),
	}, nil
}

func (s *DecomposeStrategy) decompose(task string) []string {
	lower := strings.ToLower(task)
	var tasks []string

	// Keyword-based decomposition
	if strings.Contains(lower, " and ") {
		parts := strings.Split(task, " and ")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" && len(tasks) < s.maxSubTasks {
				tasks = append(tasks, p)
			}
		}
	}

	if len(tasks) == 0 {
		// Fallback: create basic sub-tasks
		tasks = append(tasks, "analyze: "+task)
		tasks = append(tasks, "implement: "+task)
		tasks = append(tasks, "verify: "+task)
	}

	if len(tasks) > s.maxSubTasks {
		tasks = tasks[:s.maxSubTasks]
	}
	return tasks
}

func init() {
	core.RegisterStrategy("decompose", func() core.AgentStrategy {
		return NewDecomposeStrategy(core.GetStrategy("react"))
	})
}
