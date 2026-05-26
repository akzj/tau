package core

import (
	"context"
	"fmt"
	"testing"
)

func TestStrategyReAct(t *testing.T) {
	s := NewReActStrategy()
	if s.Name() != "react" {
		t.Errorf("expected 'react', got %q", s.Name())
	}
	state := AgentState{TurnCount: 0, MaxTurns: 10}
	decision, err := s.Decide(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != "continue" {
		t.Errorf("expected 'continue', got %q", decision.Action)
	}
}

func TestStrategyPlanExecute(t *testing.T) {
	s := NewPlanExecuteStrategy()
	state := AgentState{
		Messages: []Message{{Role: RoleUser, Content: "read the file main.go and analyze it"}},
		MaxTurns: 5,
	}
	decision, err := s.Decide(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != "tool:read" {
		t.Errorf("expected 'tool:read', got %q", decision.Action)
	}
}

func TestStrategyPlanExecuteReplan(t *testing.T) {
	s := NewPlanExecuteStrategy()
	state := AgentState{
		Messages:  []Message{{Role: RoleUser, Content: "read main.go"}},
		LastError: fmt.Errorf("file not found"),
		MaxTurns:  5,
	}
	decision, _ := s.Decide(context.Background(), state)
	if decision.Action == "" {
		t.Error("expected replan decision")
	}
}

func TestStrategyCoT(t *testing.T) {
	s := NewCoTStrategy()
	state := AgentState{TurnCount: 0, MaxTurns: 10}
	decision, _ := s.Decide(context.Background(), state)
	if decision.Action != "continue" {
		t.Errorf("expected 'continue', got %q", decision.Action)
	}
	state2 := AgentState{TurnCount: 1, MaxTurns: 10}
	decision2, _ := s.Decide(context.Background(), state2)
	if decision2.Action != "continue" {
		t.Errorf("expected 'continue', got %q", decision2.Action)
	}
}

func TestStrategyRegistry(t *testing.T) {
	s := GetStrategy("react")
	if s.Name() != "react" {
		t.Errorf("expected react, got %s", s.Name())
	}

	s2 := GetStrategy("nonexistent")
	if s2.Name() != "react" {
		t.Errorf("expected react default, got %s", s2.Name())
	}

	s3 := GetStrategy("plan-execute")
	if s3.Name() != "plan-execute" {
		t.Errorf("expected plan-execute, got %s", s3.Name())
	}
}
