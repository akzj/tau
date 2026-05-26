//go:build !no_plugins

package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// SubAgentSpec defines a sub-agent to spawn.
type SubAgentSpec struct {
	ID        string       // unique identifier
	Prompt    string       // initial prompt
	Tools     []string     // allowed tool names (whitelist)
	Budget    int          // max turns (default 5)
	Workspace string       // workspace root (default: /tmp/tau-sub-{id})
	Sandbox   *SandboxSpec // optional: sandbox isolation (default: enabled)
}

// SubAgentResult holds the result of a sub-agent execution.
type SubAgentResult struct {
	ID       string
	Output   string
	Err      error
	Duration time.Duration
}

// SpawnSubAgent launches a sub-agent process and returns a channel that receives the result.
func SpawnSubAgent(spec SubAgentSpec) (<-chan SubAgentResult, error) {
	if spec.Budget <= 0 {
		spec.Budget = 5
	}
	if spec.Workspace == "" {
		spec.Workspace = filepath.Join(os.TempDir(), "tau-sub-"+spec.ID)
	}
	os.MkdirAll(spec.Workspace, 0755)

	// Setup sandbox
	sandbox := spec.Sandbox
	if sandbox == nil {
		sandbox = DefaultSandbox(spec.ID)
	}
	if err := sandbox.Validate(); err != nil {
		return nil, err
	}
	if err := sandbox.Setup(); err != nil {
		return nil, err
	}

	tauBin := os.Getenv("TAU_BIN")
	if tauBin == "" {
		tauBin = "tau"
	}

	cmd := exec.Command(tauBin,
		"--max-turns", fmt.Sprintf("%d", spec.Budget),
		"--workspace", spec.Workspace,
		"--no-tools",
	)
	if err := sandbox.WrapCmd(cmd); err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	ch := make(chan SubAgentResult, 1)
	go func() {
		defer cmd.Process.Kill()
		start := time.Now()

		stdin.Write([]byte(spec.Prompt + "\n"))
		stdin.Close()

		var output []byte
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				output = append(output, buf[:n]...)
			}
			if err != nil {
				break
			}
		}

		ch <- SubAgentResult{
			ID:       spec.ID,
			Output:   string(output),
			Duration: time.Since(start),
		}
	}()

	return ch, nil
}

// AwaitSubAgent blocks until a sub-agent result is available, or ctx is cancelled.
func AwaitSubAgent(ctx context.Context, ch <-chan SubAgentResult) (SubAgentResult, error) {
	select {
	case result, ok := <-ch:
		if !ok {
			return SubAgentResult{}, fmt.Errorf("sub-agent channel closed")
		}
		return result, nil
	case <-ctx.Done():
		return SubAgentResult{}, ctx.Err()
	}
}

// SubAgentPool manages multiple concurrent sub-agents.
type SubAgentPool struct {
	mu      sync.Mutex
	results map[string]SubAgentResult
	pending map[string]<-chan SubAgentResult
	memory  *MemorySystem // optional: for memory integration
}

// NewSubAgentPool creates a pool for managing sub-agents.
func NewSubAgentPool() *SubAgentPool {
	return &SubAgentPool{
		results: make(map[string]SubAgentResult),
		pending: make(map[string]<-chan SubAgentResult),
	}
}

// Spawn launches a sub-agent in the pool.
func (p *SubAgentPool) Spawn(spec SubAgentSpec) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.pending[spec.ID]; exists {
		return fmt.Errorf("sub-agent %s already exists", spec.ID)
	}
	ch, err := SpawnSubAgent(spec)
	if err != nil {
		return err
	}
	p.pending[spec.ID] = ch
	// Memory: record sub-agent dispatch (Node 4)
	if p.memory != nil {
		p.memory.AddObservation("orchestrator",
			fmt.Sprintf("sub-agent dispatched: %s (budget: %d turns)", spec.ID, spec.Budget), 0.6)
	}
	return nil
}

// Collect blocks until all sub-agents in the pool complete.
func (p *SubAgentPool) Collect(ctx context.Context) ([]SubAgentResult, error) {
	p.mu.Lock()
	chans := make(map[string]<-chan SubAgentResult)
	for id, ch := range p.pending {
		chans[id] = ch
	}
	p.mu.Unlock()

	var results []SubAgentResult
	for id, ch := range chans {
		result, err := AwaitSubAgent(ctx, ch)
		if err != nil {
			result = SubAgentResult{ID: id, Err: err}
		}
		results = append(results, result)
	}
	// Memory: record sub-agent results (Node 5)
	if p.memory != nil {
		for _, result := range results {
			prefix := "sub-agent:" + result.ID
			if result.Err != nil {
				p.memory.AddObservation(prefix,
					fmt.Sprintf("error: %s", result.Err.Error()), 0.7)
			} else {
				p.memory.AddObservation(prefix,
					fmt.Sprintf("completed: %s", truncateStrOutput(result.Output)), 0.7)
			}
		}
	}
	return results, nil
}


func truncateStrOutput(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
