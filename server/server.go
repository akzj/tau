package server

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
)

// Server is the tau HTTP API server.
type Server struct {
	port      string
	host      string
	mux       *http.ServeMux
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
		mux:       http.NewServeMux(),
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
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/v1/health", s.handleHealth)
	s.mux.HandleFunc("/v1/tools", s.handleTools)
	s.mux.HandleFunc("/v1/sessions", s.handleSessions)
	s.mux.HandleFunc("/v1/sessions/", s.handleSessionByID)
	s.mux.HandleFunc("/v1/chat", s.handleChat)
	s.mux.HandleFunc("/v1/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("/v1/openapi.json", s.handleOpenAPI)
	s.mux.HandleFunc("/v1/admin/keys", s.handleAdminKeys)
	s.mux.HandleFunc("/v1/admin/keys/", s.handleAdminKeyByID)
}

// Start listens and serves HTTP.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	log.Printf("tau serve: http://%s (v1)", addr)
	if s.keyStore.IsDevMode() {
		log.Printf("tau serve: WARNING — no TAU_API_KEYS set, running in dev mode (no auth)")
	}
	handler := s.AuthMiddleware(s.middleware(s.mux))
	return http.ListenAndServe(addr, handler)
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