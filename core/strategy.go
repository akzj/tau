package core

import "context"

// AgentStrategy defines a pluggable reasoning strategy for the agent loop.
type AgentStrategy interface {
	Name() string
	Decide(ctx context.Context, state AgentState) (Decision, error)
}

// AgentState holds the current loop state for strategy decisions.
type AgentState struct {
	Messages     []Message
	Tools        []Tool
	TurnCount    int
	MaxTurns     int
	LastError    error
	Observations []string
}

// Decision is the output of a strategy's Decide method.
type Decision struct {
	Action   string // "tool:<name>", "continue", "stop", "wait"
	ToolName string
	ToolArgs any
	Reason   string
}

// StrategyRegistry maps strategy names to constructors.
type StrategyRegistry struct {
	strategies map[string]func() AgentStrategy
}

var globalRegistry = &StrategyRegistry{
	strategies: make(map[string]func() AgentStrategy),
}

// RegisterStrategy adds a strategy to the global registry.
func RegisterStrategy(name string, factory func() AgentStrategy) {
	globalRegistry.strategies[name] = factory
}

// GetStrategy returns a strategy by name, or ReAct as default.
func GetStrategy(name string) AgentStrategy {
	if factory, ok := globalRegistry.strategies[name]; ok {
		return factory()
	}
	if factory, ok := globalRegistry.strategies["react"]; ok {
		return factory()
	}
	return NewReActStrategy()
}
