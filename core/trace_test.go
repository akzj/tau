package core

import (
	"os"
	"testing"
	"time"
)

func TestStartSpan(t *testing.T) {
	os.Setenv("TAU_TRACE", "1")
	globalTracer.enabled = true
	defer func() { globalTracer.enabled = false; os.Unsetenv("TAU_TRACE") }()

	endSpan := StartSpan("test.operation", map[string]string{"key": "value"})
	time.Sleep(time.Millisecond)
	endSpan("ok")

	spans := globalTracer.Flush()
	if len(spans) == 0 {
		t.Error("expected at least 1 span")
	}
	found := false
	for _, s := range spans {
		if s.Name == "test.operation" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected test.operation span")
	}
}

func TestTraceDisabled(t *testing.T) {
	os.Setenv("TAU_TRACE", "0")
	globalTracer.enabled = false; globalTracer.spans = nil

	endSpan := StartSpan("disabled", nil)
	endSpan("ok")

	spans := globalTracer.Flush()
	if len(spans) > 0 {
		t.Error("expected no spans when disabled")
	}
}

func TestSpanParent(t *testing.T) {
	os.Setenv("TAU_TRACE", "1")
	globalTracer.enabled = true
	defer func() { globalTracer.enabled = false; os.Unsetenv("TAU_TRACE") }()

	endSpan1 := StartSpan("parent", nil)
	endSpan2 := StartSpan("child", nil)
	endSpan2("ok")
	endSpan1("ok")

	spans := globalTracer.Flush()
	parentCount := 0
	for _, s := range spans {
		if s.Name == "parent" {
			parentCount++
		}
	}
	if parentCount != 1 {
		t.Errorf("expected 1 parent span, got %d", parentCount)
	}
}

func TestConcurrent(t *testing.T) {
	os.Setenv("TAU_TRACE", "1")
	globalTracer.enabled = true
	defer func() { globalTracer.enabled = false; os.Unsetenv("TAU_TRACE") }()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			endSpan := StartSpan("concurrent", nil)
			endSpan("ok")
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	spans := globalTracer.Flush()
	if len(spans) < 10 {
		t.Errorf("expected >=10 spans, got %d", len(spans))
	}
}

func TestZeroOverhead(t *testing.T) {
	globalTracer.enabled = false; globalTracer.spans = nil
	start := time.Now()
	for i := 0; i < 1000; i++ {
		endSpan := StartSpan("noop", nil)
		endSpan("ok")
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("zero-overhead check: took %s for 1000 noop spans", elapsed)
	}
}