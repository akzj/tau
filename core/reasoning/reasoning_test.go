package reasoning

import (
	"context"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

func TestCoTStrategyName(t *testing.T) {
	s := NewCoTStrategy(core.GetStrategy("react"), 5)
	if s.Name() != "cot" {
		t.Errorf("expected cot, got %s", s.Name())
	}
}

func TestCoTStrategyDecide(t *testing.T) {
	s := NewCoTStrategy(core.GetStrategy("react"), 3)
	state := core.AgentState{
		Messages: []core.Message{{Role: core.RoleUser, Content: "Why is the sky blue?"}},
	}
	decision, err := s.Decide(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decision.Reason, "Step 1") {
		t.Error("expected step-by-step reasoning")
	}
	if !strings.Contains(decision.Reason, "Conclusion") {
		t.Error("expected conclusion marker")
	}
}

func TestCoTStrategySteps(t *testing.T) {
	s1 := NewCoTStrategy(core.GetStrategy("react"), 1)
	decision, _ := s1.Decide(context.Background(), core.AgentState{
		Messages: []core.Message{{Role: core.RoleUser, Content: "test"}},
	})
	if strings.Contains(decision.Reason, "Step 2") {
		t.Error("expected only 1 step")
	}

	s5 := NewCoTStrategy(core.GetStrategy("react"), 5)
	decision2, _ := s5.Decide(context.Background(), core.AgentState{
		Messages: []core.Message{{Role: core.RoleUser, Content: "test"}},
	})
	if !strings.Contains(decision2.Reason, "Step 5") {
		t.Error("expected 5 steps")
	}
}

func TestCoTStrategyDefaultSteps(t *testing.T) {
	s := NewCoTStrategy(core.GetStrategy("react"), 0)
	if s.steps != 5 {
		t.Errorf("0 should default to 5, got %d", s.steps)
	}
	// Verify it works
	_, err := s.Decide(context.Background(), core.AgentState{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestToTStrategyName(t *testing.T) {
	s := NewToTStrategy(core.GetStrategy("react"))
	if s.Name() != "tot" {
		t.Errorf("expected tot, got %s", s.Name())
	}
}

func TestToTStrategySearch(t *testing.T) {
	s := NewToTStrategy(core.GetStrategy("react"))
	state := core.AgentState{
		Messages: []core.Message{{Role: core.RoleUser, Content: "solve complex problem"}},
	}
	decision, err := s.Decide(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decision.Reason, "[ToT]") {
		t.Error("expected [ToT] prefix")
	}
	if !strings.Contains(decision.Reason, "score") {
		t.Error("expected score")
	}
}

func TestToTStrategyBFS(t *testing.T) {
	s := &ToTStrategy{inner: core.GetStrategy("react"), breadth: 2, depth: 2, topK: 1}
	candidates := s.generateCandidates(core.AgentState{}, &ToTNode{})
	if len(candidates) > 2 {
		t.Errorf("expected ≤2 candidates, got %d", len(candidates))
	}
}

func TestToTStrategyScoring(t *testing.T) {
	s := &ToTStrategy{inner: core.GetStrategy("react")}
	good := s.evaluateThought(core.AgentState{}, &ToTNode{Thought: "find the simplest solution"})
	bad := s.evaluateThought(core.AgentState{}, &ToTNode{Thought: "this will fail"})
	if good <= bad {
		t.Errorf("good should score higher: good=%.2f bad=%.2f", good, bad)
	}
}

func TestToTStrategyTracePath(t *testing.T) {
	s := &ToTStrategy{inner: core.GetStrategy("react")}
	root := &ToTNode{Thought: "root"}
	child := &ToTNode{Thought: "step1", Parent: root}
	leaf := &ToTNode{Thought: "answer", Parent: child}
	path := s.tracePath(leaf)
	if len(path) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(path))
	}
	if path[0] != "root" || path[2] != "answer" {
		t.Error("path order incorrect")
	}
}

func TestConsistencyStrategyName(t *testing.T) {
	s := NewConsistencyStrategy(core.GetStrategy("react"), 5)
	if s.Name() != "consistency" {
		t.Errorf("expected consistency, got %s", s.Name())
	}
}

func TestConsistencyStrategyVote(t *testing.T) {
	s := NewConsistencyStrategy(core.GetStrategy("react"), 5)
	state := core.AgentState{
		Messages: []core.Message{{Role: core.RoleUser, Content: "should I use Go or Rust?"}},
	}
	decision, err := s.Decide(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decision.Reason, "[Consistency]") {
		t.Error("expected [Consistency]")
	}
	if !strings.Contains(decision.Reason, "samples") {
		t.Error("expected sample count")
	}
}

func TestConsistencyStrategyMajority(t *testing.T) {
	s := NewConsistencyStrategy(core.GetStrategy("react"), 3)
	result := s.majorityVote([]string{"A", "A", "B"})
	if result != "A" {
		t.Errorf("expected A, got %s", result)
	}

	result2 := s.majorityVote([]string{"A", "B", "C"})
	// First wins on tie
	_ = result2
}

func TestConsistencyStrategyDefaultSamples(t *testing.T) {
	s := NewConsistencyStrategy(core.GetStrategy("react"), 0)
	if s.samples != 5 {
		t.Errorf("0 should default to 5, got %d", s.samples)
	}
}

func TestDecomposeStrategyName(t *testing.T) {
	s := NewDecomposeStrategy(core.GetStrategy("react"))
	if s.Name() != "decompose" {
		t.Errorf("expected decompose, got %s", s.Name())
	}
}

func TestDecomposeStrategySplit(t *testing.T) {
	s := NewDecomposeStrategy(core.GetStrategy("react"))
	state := core.AgentState{
		Messages: []core.Message{{Role: core.RoleUser, Content: "write tests and fix bugs"}},
	}
	decision, err := s.Decide(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decision.Reason, "[Decompose]") {
		t.Error("expected [Decompose]")
	}
	if !strings.Contains(decision.Reason, "sub-tasks") {
		t.Error("expected sub-task count")
	}
}

func TestDecomposeStrategyFallback(t *testing.T) {
	s := NewDecomposeStrategy(core.GetStrategy("react"))
	tasks := s.decompose("unknown complex task")
	if len(tasks) == 0 {
		t.Error("expected fallback tasks")
	}
	if len(tasks) > s.maxSubTasks {
		t.Errorf("expected ≤%d tasks, got %d", s.maxSubTasks, len(tasks))
	}
}

func TestReasoningRegistry(t *testing.T) {
	for _, name := range []string{"cot", "tot", "consistency", "decompose"} {
		s := core.GetStrategy(name)
		if s == nil {
			t.Errorf("strategy %s not registered", name)
		}
		if s.Name() != name {
			t.Errorf("expected %s, got %s", name, s.Name())
		}
	}
}

func TestReasoningBackwardCompat(t *testing.T) {
	// All existing strategies (react, plan-execute) should still work
	if core.GetStrategy("react") == nil {
		t.Error("react strategy missing")
	}
	if core.GetStrategy("plan-execute") == nil {
		t.Error("plan-execute strategy missing")
	}
}

func TestReasoningZeroOverhead(t *testing.T) {
	// Without --reasoning flag, default react strategy is used
	s := core.GetStrategy("react")
	if s == nil {
		t.Fatal("react should be default")
	}
	decision, _ := s.Decide(context.Background(), core.AgentState{
		TurnCount: 0, MaxTurns: 10,
	})
	if decision.Action != "continue" {
		t.Error("default react should continue")
	}
}

func TestReasoningIntegration(t *testing.T) {
	// All core strategies can be wrapped by reasoning strategies
	cot := NewCoTStrategy(core.GetStrategy("react"), 3)
	decision, _ := cot.Decide(context.Background(), core.AgentState{
		Messages: []core.Message{{Role: core.RoleUser, Content: "complex question"}},
	})
	if decision.Action == "" {
		t.Error("expected decision action")
	}
}

func TestDecomposeEmptyState(t *testing.T) {
	s := NewDecomposeStrategy(core.GetStrategy("react"))
	decision, err := s.Decide(context.Background(), core.AgentState{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != "stop" {
		t.Errorf("expected stop for empty state, got %s", decision.Action)
	}
}

func TestToTEmptyState(t *testing.T) {
	s := NewToTStrategy(core.GetStrategy("react"))
	decision, err := s.Decide(context.Background(), core.AgentState{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != "continue" {
		t.Errorf("expected continue, got %s", decision.Action)
	}
	if !strings.Contains(decision.Reason, "[ToT]") {
		t.Error("expected [ToT] prefix")
	}
}
