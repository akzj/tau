package core

import (
	"context"
	"strings"
)

// Planner decomposes a goal into a Plan.
type Planner interface {
	Plan(ctx context.Context, goal string) (*Plan, error)
	Name() string
}

// SimplePlanner creates a linear plan with keyword-based decomposition.
type SimplePlanner struct {
	MaxDepth int
	MaxNodes int
}

// NewSimplePlanner creates a SimplePlanner with sensible defaults.
func NewSimplePlanner() *SimplePlanner {
	return &SimplePlanner{MaxDepth: 3, MaxNodes: 20}
}

// Name returns "simple".
func (sp *SimplePlanner) Name() string { return "simple" }

// Plan decomposes a goal into a Plan tree using keyword rules.
func (sp *SimplePlanner) Plan(ctx context.Context, goal string) (*Plan, error) {
	plan := NewPlan(goal)
	plan.Strategy = "simple"

	nodes := sp.decompose(goal)
	for _, n := range nodes {
		plan.AddNode("root", &PlanNode{Title: n, Status: PlanTodo})
	}
	return plan, nil
}

func (sp *SimplePlanner) decompose(goal string) []string {
	lower := strings.ToLower(goal)

	switch {
	case strings.Contains(lower, "write") || strings.Contains(lower, "build") || strings.Contains(lower, "create"):
		return []string{
			"Analyze requirements",
			"Design architecture",
			"Implement core logic",
			"Add error handling",
			"Write tests",
			"Review and refine",
			"Document solution",
		}
	case strings.Contains(lower, "fix") || strings.Contains(lower, "debug") || strings.Contains(lower, "repair"):
		return []string{
			"Reproduce the issue",
			"Identify root cause",
			"Implement fix",
			"Verify fix with tests",
			"Check for regressions",
		}
	case strings.Contains(lower, "scrape") || strings.Contains(lower, "parse"):
		return []string{
			"Set up HTTP client",
			"Fetch target pages",
			"Parse HTML/JSON response",
			"Extract target data",
			"Handle errors and edge cases",
			"Output results",
		}
	case strings.Contains(lower, "refactor"):
		return []string{
			"Review current code",
			"Identify improvement areas",
			"Extract reusable functions",
			"Simplify conditionals",
			"Run tests to verify",
		}
	default:
		return []string{
			"Clarify requirements",
			"Break down into subtasks",
			"Execute subtasks",
			"Verify results",
			"Summarize completion",
		}
	}
}
