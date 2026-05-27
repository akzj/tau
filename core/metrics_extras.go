package core

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ── Helpers ──────────────────────────────────────────────────────────

func labelKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	return keys
}

func labelVals(labels map[string]string) []string {
	keys := labelKeys(labels)
	vals := make([]string, len(keys))
	for i, k := range keys {
		vals[i] = labels[k]
	}
	return vals
}

// ── Counter (prometheus-backed, backward-compatible API) ─────────────

// Counter is a monotonically increasing metric.
type Counter struct {
	cv     *prometheus.CounterVec
	local  atomic.Int64
	name   string
	labels map[string]string
}

// NewCounter creates a counter metric (auto-registered with prometheus).
func NewCounter(name string, labels map[string]string) *Counter {
	if labels == nil {
		labels = map[string]string{}
	}
	cv := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: name,
	}, labelKeys(labels))
	c := &Counter{cv: cv, name: name, labels: labels}
	registerExtraMetric(name, c)
	return c
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	c.cv.WithLabelValues(labelVals(c.labels)...).Inc()
	c.local.Add(1)
}

// Add adds n to the counter.
func (c *Counter) Add(n int64) {
	c.cv.WithLabelValues(labelVals(c.labels)...).Add(float64(n))
	c.local.Add(n)
}

// Value returns the current count.
func (c *Counter) Value() int64 { return c.local.Load() }

// Name returns the metric name.
func (c *Counter) Name() string { return c.name }

// Labels returns the metric labels.
func (c *Counter) Labels() map[string]string { return c.labels }

// ── Gauge (prometheus-backed, backward-compatible API) ───────────────

// Gauge is a value that goes up and down.
type Gauge struct {
	gv     *prometheus.GaugeVec
	local  atomic.Int64
	name   string
	labels map[string]string
}

// NewGauge creates a gauge metric (auto-registered with prometheus).
func NewGauge(name string, labels map[string]string) *Gauge {
	if labels == nil {
		labels = map[string]string{}
	}
	gv := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: name,
	}, labelKeys(labels))
	g := &Gauge{gv: gv, name: name, labels: labels}
	registerExtraMetric(name, g)
	return g
}

// Set sets the gauge value.
func (g *Gauge) Set(v int64) {
	g.gv.WithLabelValues(labelVals(g.labels)...).Set(float64(v))
	g.local.Store(v)
}

// Value returns the current gauge value.
func (g *Gauge) Value() int64 { return g.local.Load() }

// Name returns the metric name.
func (g *Gauge) Name() string { return g.name }

// Labels returns the metric labels.
func (g *Gauge) Labels() map[string]string { return g.labels }

// ── Histogram (prometheus-backed, backward-compatible API) ───────────

// Histogram tracks distribution of values.
type Histogram struct {
	hv     *prometheus.HistogramVec
	name   string
	labels map[string]string
	sum    atomic.Int64
	count  atomic.Int64
	minV   atomic.Int64
	maxV   atomic.Int64
}

// NewHistogram creates a histogram metric (auto-registered with prometheus).
func NewHistogram(name string, labels map[string]string) *Histogram {
	if labels == nil {
		labels = map[string]string{}
	}
	hv := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    name,
		Buckets: prometheus.DefBuckets,
	}, labelKeys(labels))
	h := &Histogram{hv: hv, name: name, labels: labels}
	h.minV.Store(1 << 60)
	registerExtraMetric(name, h)
	return h
}

// Observe adds an observation (in the same unit the caller was using).
func (h *Histogram) Observe(v int64) {
	h.hv.WithLabelValues(labelVals(h.labels)...).Observe(float64(v))
	h.count.Add(1)
	h.sum.Add(v)
	for {
		old := h.minV.Load()
		if v >= old {
			break
		}
		if h.minV.CompareAndSwap(old, v) {
			break
		}
	}
	for {
		old := h.maxV.Load()
		if v <= old {
			break
		}
		if h.maxV.CompareAndSwap(old, v) {
			break
		}
	}
}

// Value returns summary statistics.
func (h *Histogram) Value() map[string]int64 {
	c := h.count.Load()
	avg := int64(0)
	if c > 0 {
		avg = h.sum.Load() / c
	}
	return map[string]int64{
		"sum": h.sum.Load(), "count": c,
		"min": h.minV.Load(), "max": h.maxV.Load(), "avg": avg,
	}
}

// Name returns the metric name.
func (h *Histogram) Name() string { return h.name }

// Labels returns the metric labels.
func (h *Histogram) Labels() map[string]string { return h.labels }

// ── Package-level metrics (same names, prometheus-backed) ────────────

var (
	MetricRequests        = NewCounter("tau_requests_total", map[string]string{"component": "agent"})
	MetricToolCalls       = NewCounter("tau_tool_calls_total", map[string]string{"component": "agent"})
	MetricErrors          = NewCounter("tau_errors_total", map[string]string{"component": "agent"})
	MetricTokens          = NewCounter("tau_tokens_consumed_total", map[string]string{"component": "agent"})
	MetricProviderLatency = NewHistogram("tau_provider_latency_seconds", map[string]string{"component": "agent"})
)

// ── Legacy Metrics struct (for GetMetrics backward compat) ───────────

// Metrics holds named counters (backward-compatible with old GetMetrics API).
type Metrics struct {
	mu       sync.Mutex
	counters map[string]*Counter
}

// GetMetrics returns the global metrics instance (backward compat).
func GetMetrics() *Metrics {
	return globalMetrics
}

var globalMetrics = &Metrics{
	counters: make(map[string]*Counter),
}

// RegisterCounter creates a new counter (backward compat).
func (m *Metrics) RegisterCounter(name, help string) *Counter {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = make(map[string]*Counter)
	}
	c := NewCounter(name, nil)
	m.counters[name] = c
	return c
}

// ── Extra metric registry (for ExportMetrics backward compat) ────────

type extraMetric interface {
	Name() string
	Labels() map[string]string
}

var extraMetrics []extraMetric

func registerExtraMetric(name string, m extraMetric) {
	extraMetrics = append(extraMetrics, m)
}

// ExportMetrics returns all registered extra metrics in Prometheus text format.
func ExportMetrics() string {
	var out strings.Builder
	for _, m := range extraMetrics {
		name := m.Name()
		labels := m.Labels()
		labelStr := name
		for k, v := range labels {
			labelStr += fmt.Sprintf(`{%s="%s"}`, k, v)
		}
		switch v := m.(type) {
		case *Counter:
			out.WriteString(fmt.Sprintf("# TYPE %s counter\n%s %d\n", m.Name(), labelStr, v.Value()))
		case *Gauge:
			out.WriteString(fmt.Sprintf("# TYPE %s gauge\n%s %d\n", m.Name(), labelStr, v.Value()))
		case *Histogram:
			h := v.Value()
			out.WriteString(fmt.Sprintf("# TYPE %s histogram\n", m.Name()))
			for field, val := range h {
				out.WriteString(fmt.Sprintf("%s_%s %d\n", labelStr, field, val))
			}
		}
	}
	return out.String()
}