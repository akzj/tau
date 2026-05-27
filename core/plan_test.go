package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPlanCreateAndAddNodes(t *testing.T) {
	p := NewPlan("test goal")
	p.AddNode("root", &PlanNode{ID: "n1", Title: "task 1"})
	p.AddNode("root", &PlanNode{ID: "n2", Title: "task 2"})
	if len(p.Root.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(p.Root.Children))
	}
}

func TestPlanMarkStatus(t *testing.T) {
	p := NewPlan("test")
	p.AddNode("root", &PlanNode{ID: "n1", Title: "task"})
	p.MarkStatus("n1", PlanDone)
	if p.Root.Children[0].Status != PlanDone {
		t.Errorf("expected done, got %s", p.Root.Children[0].Status)
	}
}

func TestPlanGetNextTodo(t *testing.T) {
	p := NewPlan("test")
	p.AddNode("root", &PlanNode{ID: "n1", Title: "first"})
	p.AddNode("root", &PlanNode{ID: "n2", Title: "second", Dependencies: []string{"n1"}})

	next := p.GetNextTodo()
	if next == nil || next.ID != "n1" {
		t.Errorf("expected n1 as first todo, got %v", next)
	}

	p.MarkStatus("n1", PlanDone)
	next = p.GetNextTodo()
	if next == nil || next.ID != "n2" {
		t.Errorf("expected n2 after n1 done, got %v", next)
	}
}

func TestPlanDependencyBlocking(t *testing.T) {
	p := NewPlan("test")
	p.AddNode("root", &PlanNode{ID: "a", Title: "A"})
	p.AddNode("root", &PlanNode{ID: "b", Title: "B", Dependencies: []string{"a"}})

	blocked := p.GetBlocked()
	if len(blocked) != 1 || blocked[0].ID != "b" {
		t.Error("expected B blocked by A")
	}

	p.MarkStatus("a", PlanDone)
	blocked = p.GetBlocked()
	if len(blocked) != 0 {
		t.Error("expected no blocked after A done")
	}
}

func TestPlanIsComplete(t *testing.T) {
	p := NewPlan("test")
	p.AddNode("root", &PlanNode{ID: "n1", Title: "task"})
	if p.IsComplete() {
		t.Error("expected incomplete")
	}
	p.MarkStatus("n1", PlanDone)
	p.MarkStatus("root", PlanDone)
	if !p.IsComplete() {
		t.Error("expected complete")
	}
}

func TestPlanProgress(t *testing.T) {
	p := NewPlan("test")
	p.AddNode("root", &PlanNode{ID: "n1", Title: "a"})
	p.AddNode("root", &PlanNode{ID: "n2", Title: "b"})
	p.MarkStatus("n1", PlanDone)
	done, total := p.Progress()
	if done != 1 || total != 3 {
		t.Errorf("expected 1/3, got %d/%d", done, total)
	}
}

func TestPlanJSONSerialize(t *testing.T) {
	p := NewPlan("serialize test")
	p.AddNode("root", &PlanNode{ID: "s1", Title: "serialized"})
	js := p.ToJSON()
	var decoded Plan
	if err := json.Unmarshal([]byte(js), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Goal != "serialize test" {
		t.Error("goal mismatch after serialize")
	}
}

func TestPlanVisualize(t *testing.T) {
	p := NewPlan("visualize")
	p.AddNode("root", &PlanNode{ID: "v1", Title: "visible task"})
	p.MarkStatus("v1", PlanDone)
	viz := p.Visualize()
	if !strings.Contains(viz, "✅") {
		t.Error("expected ✅ icon")
	}
	if !strings.Contains(viz, "visible task") {
		t.Error("expected task title")
	}
}

func TestSimplePlanner(t *testing.T) {
	sp := NewSimplePlanner()
	plan, err := sp.Plan(context.Background(), "write a web scraper in Python")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Root.Children) == 0 {
		t.Error("expected decomposed nodes")
	}
	if !strings.Contains(plan.ToJSON(), "scraper") {
		t.Error("goal not in JSON")
	}
}

func TestPlanLinearFallback(t *testing.T) {
	sp := NewSimplePlanner()
	plan, _ := sp.Plan(context.Background(), "xyzzy unknown task")
	if len(plan.Root.Children) == 0 {
		t.Error("expected linear fallback for unknown goal")
	}
}

func TestPlanBackwardCompat(t *testing.T) {
	p := NewPlan("solo")
	p.MarkStatus("root", PlanDone)
	if !p.IsComplete() {
		t.Error("solo plan should be complete")
	}
}

func TestPlanZeroRegression(t *testing.T) {
	p := NewPlan("regression test")
	p.AddNode("root", &PlanNode{ID: "r1", Title: "test", Status: PlanDone})
	if v := p.Visualize(); v == "" {
		t.Error("visualize should not be empty")
	}
	if p.GetNextTodo() != nil {
		t.Error("no next todo when all done")
	}
}
