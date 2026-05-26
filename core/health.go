package core

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthStatus represents the health check response.
type HealthStatus struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
	Providers int    `json:"providers"`
	Tools     int    `json:"tools"`
}

// Version is set at build time via ldflags. Default: "dev".
var Version = "dev"

// StartTime records when the application started.
var StartTime = time.Now()

// HealthCheck returns the current health status.
func HealthCheck(providerCount, toolCount int) HealthStatus {
	return HealthStatus{
		Status:    "ok",
		Version:   Version,
		Uptime:    time.Since(StartTime).Round(time.Second).String(),
		Providers: providerCount,
		Tools:     toolCount,
	}
}

// HealthHandler returns an http.Handler that serves the health check endpoint.
func HealthHandler(providerCount, toolCount int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := HealthCheck(providerCount, toolCount)
		json.NewEncoder(w).Encode(status)
	}
}

// ReadyHandler returns an http.Handler that always returns 200 OK.
func ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	}
}
