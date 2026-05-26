package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EvalScenario defines a single evaluation test case.
type EvalScenario struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Expected    []string `json:"expected"`
	Forbidden   []string `json:"forbidden"`
	Tools       []string `json:"tools"`
	MaxTurns    int      `json:"max_turns"`
}

// EvalResult holds the outcome of a single evaluation.
type EvalResult struct {
	Scenario    string  `json:"scenario"`
	Passed      bool    `json:"passed"`
	Correctness float64 `json:"correctness"`
	ToolScore   float64 `json:"tool_score"`
	Latency     float64 `json:"latency"`
	Total       float64 `json:"total"`
	Error       string  `json:"error,omitempty"`
	Duration    string  `json:"duration"`
}

// EvalSuite holds a collection of scenarios.
type EvalSuite struct {
	Name      string         `json:"name"`
	Scenarios []EvalScenario `json:"scenarios"`
}

// LoadSuite loads an evaluation suite from a directory of .json files.
func LoadSuite(dir string) (*EvalSuite, error) {
	suite := &EvalSuite{Name: filepath.Base(dir)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var scenario EvalScenario
		if err := json.Unmarshal(data, &scenario); err != nil {
			continue
		}
		if scenario.MaxTurns <= 0 {
			scenario.MaxTurns = 3
		}
		suite.Scenarios = append(suite.Scenarios, scenario)
	}
	return suite, nil
}

// RunEval runs an evaluation scenario with a stream function.
func RunEval(scenario EvalScenario, streamFn func(ctx context.Context, prompt string) (<-chan ProviderEvent, error)) EvalResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := EvalResult{
		Scenario:    scenario.Name,
		Correctness: 1.0,
		ToolScore:   1.0,
	}

	events, err := streamFn(ctx, scenario.Prompt)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start).String()
		return result
	}

	var response strings.Builder
	toolsUsed := make(map[string]bool)
	for ev := range events {
		if ev.Type == ProvContentDelta {
			response.WriteString(ev.ContentDelta)
		}
		if ev.Type == ProvToolCallStart {
			toolsUsed[ev.ToolName] = true
		}
	}

	text := response.String()

	correctCount := 0
	for _, exp := range scenario.Expected {
		if strings.Contains(strings.ToLower(text), strings.ToLower(exp)) {
			correctCount++
		}
	}
	if len(scenario.Expected) > 0 {
		result.Correctness = float64(correctCount) / float64(len(scenario.Expected))
	}

	for _, fb := range scenario.Forbidden {
		if strings.Contains(strings.ToLower(text), strings.ToLower(fb)) {
			result.Correctness = 0.0
			result.Error = fmt.Sprintf("found forbidden: %q", fb)
			break
		}
	}

	if len(scenario.Tools) > 0 {
		correctTools := 0
		for _, t := range scenario.Tools {
			if toolsUsed[t] {
				correctTools++
			}
		}
		result.ToolScore = float64(correctTools) / float64(len(scenario.Tools))
	}

	elapsed := time.Since(start)
	result.Duration = elapsed.Round(time.Millisecond).String()
	if elapsed < 100*time.Millisecond {
		result.Latency = 1.0
	} else if elapsed < time.Second {
		result.Latency = 0.8
	} else if elapsed < 5*time.Second {
		result.Latency = 0.5
	} else {
		result.Latency = 0.2
	}

	result.Total = result.Correctness*0.5 + result.ToolScore*0.3 + result.Latency*0.2
	result.Passed = result.Correctness >= 0.8 && result.Error == ""

	return result
}

// EvalReport generates a Markdown table from evaluation results.
func EvalReport(results []EvalResult) string {
	var b strings.Builder
	b.WriteString("## Evaluation Report\n\n")
	b.WriteString("| Scenario | Pass | Correctness | Tools | Latency | Total | Duration |\n")
	b.WriteString("|----------|------|-------------|-------|---------|-------|----------|\n")

	passed := 0
	var totalScore float64
	for _, r := range results {
		icon := "❌"
		if r.Passed {
			icon = "✅"
			passed++
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %.2f | %.2f | %s |\n",
			r.Scenario, icon, r.Correctness, r.ToolScore, r.Latency, r.Total, r.Duration))
		totalScore += r.Total
	}

	avgScore := totalScore / float64(maxInt(len(results), 1))
	b.WriteString(fmt.Sprintf("\n**Passed**: %d/%d | **Avg Score**: %.2f\n", passed, len(results), avgScore))
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Ensure imports used
var _ = sort.Strings
var _ = json.Marshal
var _ = fmt.Sprintf
