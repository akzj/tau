package server

import (
	"os"

	"github.com/akzj/tau/core"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) handleTelemetryTraces(c *gin.Context) {
	if os.Getenv("TAU_TELEMETRY_ENABLED") != "1" {
		c.JSON(403, gin.H{"error": "telemetry not enabled — set TAU_TELEMETRY_ENABLED=1"})
		return
	}
	spans := core.GetTracer().Flush()
	c.JSON(200, gin.H{"spans": spans, "count": len(spans)})
}

func (s *Server) handleTelemetryMetrics(c *gin.Context) {
	if os.Getenv("TAU_TELEMETRY_ENABLED") != "1" {
		c.JSON(403, gin.H{"error": "telemetry not enabled"})
		return
	}

	// Gather from Prometheus registry to build JSON response.
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to gather metrics"})
		return
	}

	metrics := gin.H{}
	for _, mf := range mfs {
		name := mf.GetName()
		for _, m := range mf.GetMetric() {
			switch {
			case m.GetCounter() != nil:
				metrics[name] = m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				metrics[name] = m.GetGauge().GetValue()
			case m.GetHistogram() != nil:
				h := m.GetHistogram()
				metrics[name+"_count"] = h.GetSampleCount()
				metrics[name+"_sum"] = h.GetSampleSum()
			}
		}
	}
	c.JSON(200, gin.H{"metrics": metrics})
}

func (s *Server) handleMetrics(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}