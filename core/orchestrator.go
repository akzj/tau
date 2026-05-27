package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DelegationTask represents a single task to delegate to a sub-agent.
type DelegationTask struct {
	ID       string        `json:"id"`
	Prompt   string        `json:"prompt"`
	Context  string        `json:"context,omitempty"`
	MaxTurns int           `json:"max_turns"`
	Timeout  time.Duration `json:"timeout"`
}

// DelegationResult holds the result of a delegated task.
type DelegationResult struct {
	TaskID    string    `json:"task_id"`
	Response  string    `json:"response"`
	Tokens    int       `json:"tokens"`
	ToolCalls int       `json:"tool_calls"`
	Error     string    `json:"error,omitempty"`
	Duration  string    `json:"duration"`
	StartTime time.Time `json:"start_time"`
}

// AgentPool manages a pool of reusable agent instances.
type AgentPool struct {
	size     int
	provider Provider
	model    ModelSpec
}

// NewAgentPool creates a pool of N agent instances sharing a provider.
func NewAgentPool(size int, provider Provider, model ModelSpec) *AgentPool {
	if size <= 0 {
		size = 1
	}
	return &AgentPool{size: size, provider: provider, model: model}
}

// Delegate submits a batch of tasks and collects results.
// Each task runs in its own goroutine. One failure does not block others.
func (p *AgentPool) Delegate(ctx context.Context, tasks []DelegationTask) []DelegationResult {
	var wg sync.WaitGroup
	results := make([]DelegationResult, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t DelegationTask) {
			defer wg.Done()
			results[idx] = p.runTask(ctx, t)
		}(i, task)
	}
	wg.Wait()
	return results
}

// Collect is an alias for Delegate for API symmetry.
func (p *AgentPool) Collect(ctx context.Context, tasks []DelegationTask) []DelegationResult {
	return p.Delegate(ctx, tasks)
}

// Size returns the pool size.
func (p *AgentPool) Size() int { return p.size }

func (p *AgentPool) runTask(ctx context.Context, task DelegationTask) DelegationResult {
	start := time.Now()
	result := DelegationResult{TaskID: task.ID, StartTime: start}

	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}
	if task.MaxTurns <= 0 {
		task.MaxTurns = 5
	}

	prompt := task.Prompt
	if task.Context != "" {
		prompt = fmt.Sprintf("%s\n\nContext: %s", task.Prompt, task.Context)
	}

	sess, err := NewSession(ctx, SessionOptions{
		Provider:     p.provider,
		DefaultModel: p.model,
	})
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start).String()
		return result
	}
	_ = task.MaxTurns // turn limit per task

	loop := NewLoop()
	run, err := loop.Prompt(ctx, sess, UserInput{Text: prompt})
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start).String()
		return result
	}

	var response string
	toolCalls := 0
	for ev := range run.Events() {
		switch msg := ev.(type) {
		case MessageDelta:
			response += msg.ContentDelta
		case ToolCallStart:
			toolCalls++
		case ErrorEvent:
			if result.Error == "" {
				result.Error = msg.Err.Error()
			}
		}
	}

	result.Response = response
	result.ToolCalls = toolCalls
	result.Tokens = CountTokens(prompt) + CountTokens(response)
	result.Duration = time.Since(start).String()
	run.Done()
	return result
}
