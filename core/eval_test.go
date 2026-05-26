package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEvalPass(t *testing.T) {
	scenario := EvalScenario{
		Name:     "hello-test",
		Prompt:   "say hello",
		Expected: []string{"hello"},
	}
	result := RunEval(scenario, func(ctx context.Context, prompt string) (<-chan ProviderEvent, error) {
		ch := make(chan ProviderEvent, 1)
		go func() {
			ch <- ProviderEvent{Type: ProvContentDelta, ContentDelta: "hello world"}
			close(ch)
		}()
		return ch, nil
	})
	if !result.Passed {
		t.Errorf("expected pass, got %+v", result)
	}
	if result.Correctness < 0.8 {
		t.Errorf("correctness too low: %f", result.Correctness)
	}
}

func TestRunEvalFailMissing(t *testing.T) {
	scenario := EvalScenario{
		Name:     "missing-test",
		Prompt:   "say hi",
		Expected: []string{"goodbye"},
	}
	result := RunEval(scenario, func(ctx context.Context, prompt string) (<-chan ProviderEvent, error) {
		ch := make(chan ProviderEvent, 1)
		go func() {
			ch <- ProviderEvent{Type: ProvContentDelta, ContentDelta: "hello"}
			close(ch)
		}()
		return ch, nil
	})
	if result.Passed {
		t.Error("expected fail for missing expected substring")
	}
}

func TestRunEvalFailForbidden(t *testing.T) {
	scenario := EvalScenario{
		Name:      "forbidden-test",
		Prompt:    "anything",
		Forbidden: []string{"secret"},
	}
	result := RunEval(scenario, func(ctx context.Context, prompt string) (<-chan ProviderEvent, error) {
		ch := make(chan ProviderEvent, 1)
		go func() {
			ch <- ProviderEvent{Type: ProvContentDelta, ContentDelta: "the secret is 12345"}
			close(ch)
		}()
		return ch, nil
	})
	if result.Passed {
		t.Error("expected fail for forbidden substring")
	}
}

func TestEvalReport(t *testing.T) {
	results := []EvalResult{
		{Scenario: "test1", Passed: true, Correctness: 0.9, ToolScore: 0.8, Latency: 0.7, Total: 0.82, Duration: "10ms"},
		{Scenario: "test2", Passed: false, Correctness: 0.3, Total: 0.3, Duration: "5ms"},
	}
	report := EvalReport(results)
	if !strings.Contains(report, "test1") {
		t.Error("expected test1 in report")
	}
	if !strings.Contains(report, "test2") {
		t.Error("expected test2 in report")
	}
}

func TestLoadSuite(t *testing.T) {
	dir := t.TempDir()
	scenario := `{"name":"test","prompt":"hello","expected":["hi"]}`
	os.WriteFile(filepath.Join(dir, "test.json"), []byte(scenario), 0644)
	suite, err := LoadSuite(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Scenarios) != 1 {
		t.Errorf("expected 1 scenario, got %d", len(suite.Scenarios))
	}
}
