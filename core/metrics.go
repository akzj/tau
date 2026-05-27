package core

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ── Direct prometheus metrics (tau core-level) ───────────────────────

var (
	metricSessionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tau_session_duration_seconds",
		Help:    "Session duration in seconds.",
		Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
	})
)

// RegisterTauMetrics initialises the standard tau metrics.
// Counters go through GetMetrics() so they appear in the legacy counters map.
// Note: tau_tool_calls_total is NOT registered here — it is handled by the
// package-level MetricToolCalls (metrics_extras.go) which is prometheus-backed.
func RegisterTauMetrics() {
	globalMetrics.RegisterCounter("tau_turns_total", "Total number of agent turns.")
	globalMetrics.RegisterCounter("tau_compact_operations_total", "Total compaction operations.")
	// Histogram registered directly with prometheus.
	metricSessionDuration.Observe(0)
}

// MetricsHandler returns an http.Handler that serves /metrics in Prometheus text format.
func MetricsHandler() http.HandlerFunc {
	return promhttp.Handler().ServeHTTP
}