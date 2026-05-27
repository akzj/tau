package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// PlanStatus tracks the state of a plan node.
type PlanStatus string

const (
	PlanTodo       PlanStatus = "todo"
	PlanInProgress PlanStatus = "in_progress"
	PlanDone       PlanStatus = "done"
	PlanFailed     PlanStatus = "failed"
	PlanSkipped    PlanStatus = "skipped"
)

// PlanNode is a single node in the plan tree.
type PlanNode struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	Description  string      `json:"description,omitempty"`
	Status       PlanStatus  `json:"status"`
	Dependencies []string    `json:"dependencies,omitempty"` // node IDs that must be done first
	Children     []*PlanNode `json:"children,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
}

// Plan is the complete task decomposition.
type Plan struct {
	mu       sync.RWMutex `json:"-"`
	ID       string       `json:"id"`
	Goal     string       `json:"goal"`
	Root     *PlanNode    `json:"root"`
	Created  string       `json:"created"`
	Strategy string       `json:"strategy"`
}

// NewPlan creates an empty plan for a goal.
func NewPlan(goal string) *Plan {
	return &Plan{
		ID:       fmt.Sprintf("plan-%d", time.Now().UnixNano()),
		Goal:     goal,
		Root:     &PlanNode{ID: "root", Title: goal, Status: PlanInProgress},
		Created:  time.Now().Format(time.RFC3339),
		Strategy: "simple",
	}
}

// AddNode adds a child node to a parent.
func (p *Plan) AddNode(parentID string, node *PlanNode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	parent := p.findNode(p.Root, parentID)
	if parent != nil {
		if node.ID == "" {
			node.ID = fmt.Sprintf("%s.%d", parent.ID, len(parent.Children)+1)
		}
		if node.Status == "" {
			node.Status = PlanTodo
		}
		parent.Children = append(parent.Children, node)
	}
}

// MarkStatus updates a node's status.
func (p *Plan) MarkStatus(nodeID string, status PlanStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	node := p.findNode(p.Root, nodeID)
	if node != nil {
		node.Status = status
	}
}

// GetNextTodo returns the next actionable node (dependencies satisfied, status=Todo).
func (p *Plan) GetNextTodo() *PlanNode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nextTodo(p.Root)
}

// GetBlocked returns nodes blocked by incomplete dependencies.
func (p *Plan) GetBlocked() []*PlanNode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var blocked []*PlanNode
	p.collectBlocked(p.Root, &blocked)
	return blocked
}

// IsComplete returns true if all nodes are done/skipped/failed.
func (p *Plan) IsComplete() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.allDone(p.Root)
}

// Progress returns completed/total ratio.
func (p *Plan) Progress() (done, total int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.countNodes(p.Root, &done, &total)
	return
}

// ToJSON serializes the plan to JSON.
func (p *Plan) ToJSON() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	data, _ := json.MarshalIndent(p, "", "  ")
	return string(data)
}

// Visualize returns a text tree representation.
func (p *Plan) Visualize() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Plan: %s [%s]\n", p.Goal, p.ID))
	p.renderNode(p.Root, &b, "", true)
	return b.String()
}

// --- helpers ---

func (p *Plan) findNode(n *PlanNode, id string) *PlanNode {
	if n == nil {
		return nil
	}
	if n.ID == id {
		return n
	}
	for _, c := range n.Children {
		if found := p.findNode(c, id); found != nil {
			return found
		}
	}
	return nil
}

func (p *Plan) nextTodo(n *PlanNode) *PlanNode {
	if n == nil {
		return nil
	}
	if n.Status == PlanTodo && p.depsSatisfied(n) {
		return n
	}
	for _, c := range n.Children {
		if found := p.nextTodo(c); found != nil {
			return found
		}
	}
	return nil
}

func (p *Plan) depsSatisfied(n *PlanNode) bool {
	for _, depID := range n.Dependencies {
		dep := p.findNode(p.Root, depID)
		if dep == nil || dep.Status != PlanDone {
			return false
		}
	}
	return true
}

func (p *Plan) collectBlocked(n *PlanNode, blocked *[]*PlanNode) {
	if n == nil {
		return
	}
	if n.Status == PlanTodo && !p.depsSatisfied(n) {
		*blocked = append(*blocked, n)
	}
	for _, c := range n.Children {
		p.collectBlocked(c, blocked)
	}
}

func (p *Plan) allDone(n *PlanNode) bool {
	if n == nil {
		return true
	}
	if n.Status != PlanDone && n.Status != PlanSkipped && n.Status != PlanFailed {
		return false
	}
	for _, c := range n.Children {
		if !p.allDone(c) {
			return false
		}
	}
	return true
}

func (p *Plan) countNodes(n *PlanNode, done, total *int) {
	if n == nil {
		return
	}
	*total++
	if n.Status == PlanDone {
		*done++
	}
	for _, c := range n.Children {
		p.countNodes(c, done, total)
	}
}

func (p *Plan) renderNode(n *PlanNode, b *strings.Builder, prefix string, last bool) {
	if n == nil {
		return
	}
	connector := "├── "
	if last {
		connector = "└── "
	}
	if n == p.Root {
		connector = ""
	}

	icon := map[PlanStatus]string{
		PlanTodo: "⏳", PlanInProgress: "🔄", PlanDone: "✅", PlanFailed: "❌", PlanSkipped: "⏭️",
	}[n.Status]
	b.WriteString(fmt.Sprintf("%s%s%s %s\n", prefix, connector, icon, n.Title))

	newPrefix := prefix
	if n != p.Root {
		if last {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}
	for i, c := range n.Children {
		p.renderNode(c, b, newPrefix, i == len(n.Children)-1)
	}
}
