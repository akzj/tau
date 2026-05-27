package core

import (
	"os"
	"testing"
	"strings"
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
func TestSpanTime(t *testing.T) {
	os.Setenv("TAU_TRACE", "1")
	globalTracer.enabled = true
	defer func() { globalTracer.enabled = false; os.Unsetenv("TAU_TRACE"); globalTracer.spans = nil }()

	endSpan := StartSpan("timed", nil)
	time.Sleep(10 * time.Millisecond)
	endSpan("ok")

	spans := globalTracer.Flush()
	if len(spans) == 0 {
		t.Error("expected spans")
	}
	for _, s := range spans {
		if s.Name == "timed" && s.Duration == "" {
			t.Error("expected non-empty duration")
		}
	}
}

func TestSpanErrorStatus(t *testing.T) {
	os.Setenv("TAU_TRACE", "1")
	globalTracer.enabled = true
	defer func() { globalTracer.enabled = false; os.Unsetenv("TAU_TRACE"); globalTracer.spans = nil }()

	endSpan := StartSpan("failing", nil)
	endSpan("error")

	spans := globalTracer.Flush()
	for _, s := range spans {
		if s.Name == "failing" && s.Status != "error" {
			t.Errorf("expected status=error, got %s", s.Status)
		}
	}
}

func TestCounterConcurrent(t *testing.T) {
	c := NewCounter("concurrent_counter", nil)
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() { c.Inc(); done <- true }()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
	if c.Value() != 100 {
		t.Errorf("expected 100, got %d", c.Value())
	}
}

func TestGaugeConcurrent(t *testing.T) {
	g := NewGauge("concurrent_gauge", nil)
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func(v int64) { g.Set(v); done <- true }(int64(i))
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	_ = g.Value()
}

func TestHistogramConcurrent(t *testing.T) {
	h := NewHistogram("concurrent_hist", nil)
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func(v int64) { h.Observe(v); done <- true }(int64(i))
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	v := h.Value()
	if v["count"] != 50 {
		t.Errorf("expected count=50, got %d", v["count"])
	}
}

func TestExportMetricsFormat(t *testing.T) {
	output := ExportMetrics()
	if !strings.Contains(output, "# TYPE") {
		t.Error("expected Prometheus # TYPE lines")
	}
	if !strings.Contains(output, "tau_requests_total") {
		t.Error("expected tau_requests_total metric")
	}
}

func TestTelemetryEnabled(t *testing.T) {
	os.Setenv("TAU_TRACE", "0")
	globalTracer.enabled = false
	if TraceEnabled() {
		t.Error("expected disabled")
	}

	os.Setenv("TAU_TRACE", "1")
	globalTracer.enabled = true
	if !TraceEnabled() {
		t.Error("expected enabled")
	}
	globalTracer.enabled = false
}
