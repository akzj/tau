package core

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Metric types for manual Prometheus format generation.

type counterMetric struct {
	mu     sync.Mutex
	name   string
	help   string
	labels map[string]map[string]int64
	count  int64
}

type histogramMetric struct {
	mu      sync.Mutex
	name    string
	help    string
	buckets []float64
	counts  []int64
	sum     float64
	count   int64
}

// Metrics holds all tau metrics.
type Metrics struct {
	mu         sync.Mutex
	counters   map[string]*counterMetric
	histograms map[string]*histogramMetric
}

var globalMetrics = &Metrics{
	counters:   make(map[string]*counterMetric),
	histograms: make(map[string]*histogramMetric),
}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics { return globalMetrics }

// RegisterCounter creates a new counter.
func (m *Metrics) RegisterCounter(name, help string) *counterMetric {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := &counterMetric{name: name, help: help, labels: make(map[string]map[string]int64)}
	m.counters[name] = c
	return c
}

// Inc increments a counter.
func (c *counterMetric) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

// RegisterHistogram creates a new histogram.
func (m *Metrics) RegisterHistogram(name, help string, buckets []float64) *histogramMetric {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := &histogramMetric{name: name, help: help, buckets: buckets, counts: make([]int64, len(buckets))}
	m.histograms[name] = h
	return h
}

// Observe adds an observation to the histogram.
func (h *histogramMetric) Observe(val float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sum += val
	h.count++
	for i, b := range h.buckets {
		if val <= b {
			h.counts[i]++
		}
	}
}

// RegisterTauMetrics initializes the 4 standard tau metrics.
func RegisterTauMetrics() {
	globalMetrics.RegisterCounter("tau_turns_total", "Total number of agent turns.")
	globalMetrics.RegisterCounter("tau_tool_calls_total", "Total number of tool calls.")
	globalMetrics.RegisterCounter("tau_compact_operations_total", "Total compaction operations.")
	globalMetrics.RegisterHistogram("tau_session_duration_seconds", "Session duration in seconds.",
		[]float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600})
}

// MetricsHandler returns an http.Handler that serves /metrics in Prometheus text format.
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		var b strings.Builder
		globalMetrics.mu.Lock()
		defer globalMetrics.mu.Unlock()
		for _, c := range globalMetrics.counters {
			b.WriteString(fmt.Sprintf("# HELP %s %s\n", c.name, c.help))
			b.WriteString(fmt.Sprintf("# TYPE %s counter\n", c.name))
			b.WriteString(fmt.Sprintf("%s %d\n", c.name, c.count))
		}
		for _, h := range globalMetrics.histograms {
			b.WriteString(fmt.Sprintf("# HELP %s %s\n", h.name, h.help))
			b.WriteString(fmt.Sprintf("# TYPE %s histogram\n", h.name))
			b.WriteString(fmt.Sprintf("%s_sum %f\n", h.name, h.sum))
			b.WriteString(fmt.Sprintf("%s_count %d\n", h.name, h.count))
			for i, bucket := range h.buckets {
				b.WriteString(fmt.Sprintf("%s_bucket{le=\"%f\"} %d\n", h.name, bucket, h.counts[i]))
			}
			b.WriteString(fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", h.name, h.count))
		}
		w.Write([]byte(b.String()))
	}
}
