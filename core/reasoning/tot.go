package reasoning

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/akzj/tau/core"
)

// ToTNode is a node in the tree-of-thought search.
type ToTNode struct {
	Thought  string
	Score    float64
	Parent   *ToTNode
	Children []*ToTNode
}

// ToTStrategy implements Tree-of-Thoughts search.
// Breadth=B candidate next thoughts, evaluate each, keep top-K, recurse depth=D.
type ToTStrategy struct {
	inner   core.AgentStrategy
	breadth int // number of candidates per step
	depth   int // max recursion depth
	topK    int // keep top K at each level
}

// NewToTStrategy creates a Tree-of-Thoughts strategy with defaults (breadth=3, depth=3, topK=2).
func NewToTStrategy(inner core.AgentStrategy) *ToTStrategy {
	return &ToTStrategy{inner: inner, breadth: 3, depth: 3, topK: 2}
}

// Name returns "tot".
func (s *ToTStrategy) Name() string { return "tot" }

// Decide runs the ToT search and returns the best reasoning path found.
func (s *ToTStrategy) Decide(ctx context.Context, state core.AgentState) (core.Decision, error) {
	root := &ToTNode{Thought: "root", Score: 1.0}
	s.search(ctx, state, root, 0)

	// Find best leaf
	best := s.bestLeaf(root)
	path := s.tracePath(best)

	return core.Decision{
		Action: "continue",
		Reason: fmt.Sprintf("[ToT] best path: %s (score: %.2f)", strings.Join(path, " → "), best.Score),
	}, nil
}

func (s *ToTStrategy) search(ctx context.Context, state core.AgentState, node *ToTNode, currentDepth int) {
	if currentDepth >= s.depth {
		return
	}

	// Generate B candidates
	candidates := s.generateCandidates(state, node)

	// Score each candidate
	for _, c := range candidates {
		c.Score = s.evaluateThought(state, c)
	}

	// Sort by score, keep top-K
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	if len(candidates) > s.topK {
		candidates = candidates[:s.topK]
	}

	node.Children = candidates
	for _, c := range candidates {
		s.search(ctx, state, c, currentDepth+1)
	}
}

func (s *ToTStrategy) generateCandidates(state core.AgentState, parent *ToTNode) []*ToTNode {
	templates := []string{
		"What are the key facts?",
		"What approach could work?",
		"What are the constraints?",
		"What's the simplest solution?",
		"What could go wrong?",
	}
	var nodes []*ToTNode
	for i, t := range templates {
		if i >= s.breadth {
			break
		}
		nodes = append(nodes, &ToTNode{Thought: t, Parent: parent})
	}
	return nodes
}

func (s *ToTStrategy) evaluateThought(state core.AgentState, node *ToTNode) float64 {
	score := 0.5 // base
	lower := strings.ToLower(node.Thought)
	if strings.Contains(lower, "solution") || strings.Contains(lower, "approach") {
		score += 0.3
	}
	if strings.Contains(lower, "simple") {
		score += 0.2
	}
	if strings.Contains(lower, "wrong") || strings.Contains(lower, "fail") {
		score -= 0.2
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

func (s *ToTStrategy) bestLeaf(node *ToTNode) *ToTNode {
	if len(node.Children) == 0 {
		return node
	}
	best := node.Children[0]
	for _, c := range node.Children[1:] {
		if c.Score > best.Score {
			best = c
		}
	}
	return s.bestLeaf(best)
}

func (s *ToTStrategy) tracePath(node *ToTNode) []string {
	var path []string
	for n := node; n != nil; n = n.Parent {
		path = append([]string{n.Thought}, path...)
	}
	return path
}

func init() {
	core.RegisterStrategy("tot", func() core.AgentStrategy {
		return NewToTStrategy(core.GetStrategy("react"))
	})
}
