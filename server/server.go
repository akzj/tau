package server

import (
	"fmt"
	"log"
	"sync"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Server is the tau HTTP API server.
type Server struct {
	port      string
	host      string
	router    *gin.Engine
	sessions  map[string]*coding.CodingSession
	mu        sync.RWMutex
	workspace string
	provider  core.Provider
	model     string
	wireAPI   core.WireAPI
	keyStore  *APIKeyStore
}

// Options configures a new Server.
type Options struct {
	Port      string
	Host      string
	APIKey    string // legacy: single API key (added to keyStore if set)
	Workspace string
	Provider  core.Provider
	Model     string
	WireAPI   core.WireAPI
}

// New creates a new Server.
func New(opts Options) *Server {
	s := &Server{
		port:      opts.Port,
		host:      opts.Host,
		router:    gin.New(),
		sessions:  make(map[string]*coding.CodingSession),
		workspace: opts.Workspace,
		provider:  opts.Provider,
		model:     opts.Model,
		wireAPI:   opts.WireAPI,
		keyStore:  NewAPIKeyStore(),
	}
	// Legacy: if a single API key was passed via Options, add it to the key store.
	if opts.APIKey != "" {
		s.keyStore.AddKey(opts.APIKey)
		// If TAU_API_KEYS env wasn't set, we're in dev mode — adding a key
		// explicitly via Options should disable dev mode.
		s.keyStore.devMode = false
	}
	s.setupMiddleware()
	s.setupRoutes()
	return s
}

// ── Prometheus middleware metrics ────────────────────────────────

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tau_http_requests_total",
		Help: "Total HTTP requests handled by tau server.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "tau_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		timer := prometheus.NewTimer(httpRequestDuration.WithLabelValues(
			c.Request.Method, c.FullPath(),
		))
		defer timer.ObserveDuration()

		c.Next()

		httpRequestsTotal.WithLabelValues(
			c.Request.Method, c.FullPath(),
			fmt.Sprintf("%d", c.Writer.Status()),
		).Inc()
	}
}

func (s *Server) setupMiddleware() {
	s.router.Use(gin.Logger())
	s.router.Use(gin.Recovery())
	s.router.Use(prometheusMiddleware())
	s.router.Use(s.corsMiddleware())
	s.router.Use(s.authMiddleware())
}

func (s *Server) setupRoutes() {
	// Public
	s.router.GET("/v1/health", s.handleHealth)
	s.router.GET("/v1/openapi.json", s.handleOpenAPI)
	s.router.GET("/metrics", s.handleMetrics)
	s.router.GET("/v1/telemetry/traces", s.handleTelemetryTraces)
	s.router.GET("/v1/telemetry/metrics", s.handleTelemetryMetrics)

	// Tools
	s.router.GET("/v1/tools", s.handleTools)

	// Sessions
	s.router.GET("/v1/sessions", s.handleListSessions)
	s.router.POST("/v1/sessions", s.handleCreateSession)
	s.router.GET("/v1/sessions/:id", s.handleGetSession)
	s.router.DELETE("/v1/sessions/:id", s.handleDeleteSession)

	// Chat
	s.router.POST("/v1/chat", s.handleChat)
	s.router.POST("/v1/chat/stream", s.handleChatStream)

	// Admin (auth middleware handles admin key check)
	s.router.GET("/v1/admin/keys", s.handleAdminKeys)
	s.router.POST("/v1/admin/keys", s.handleAdminKeys)
	s.router.DELETE("/v1/admin/keys/:id", s.handleAdminKeyByID)
}

// Start listens and serves HTTP.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	log.Printf("tau serve: http://%s (v1) [gin]", addr)
	if s.keyStore.IsDevMode() {
		log.Printf("tau serve: WARNING — no TAU_API_KEYS set, running in dev mode (no auth)")
	}
	return s.router.Run(addr)
}

func (s *Server) getSession(id string) (*coding.CodingSession, error) {
	s.mu.RLock()
	sess := s.sessions[id]
	s.mu.RUnlock()
	if sess == nil {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return sess, nil
}
