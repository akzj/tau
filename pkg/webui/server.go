package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/persist"
)

//go:embed templates/*
var templateFS embed.FS

// Server handles HTTP and WebSocket connections for the tau web UI.
type Server struct {
	addr      string
	workspace string
	model     string
	provider  core.Provider
	upgrader  websocket.Upgrader
	wsMu      sync.Mutex // protects concurrent WebSocket writes
	activeRun context.CancelFunc
	runMu     sync.Mutex // protects activeRun
}

// NewServer creates a webui server.
func NewServer(addr, workspace, model string, prov core.Provider) *Server {
	return &Server{
		addr:      addr,
		workspace: workspace,
		model:     model,
		provider:  prov,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Start begins listening with graceful shutdown on SIGINT.
func (s *Server) Start() error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/sessions", s.handleSessionList)

	srv := &http.Server{Addr: s.addr, Handler: nil}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	fmt.Fprintf(os.Stderr, "tau webui: http://%s\n", s.addr)
	return srv.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := templateFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "template not found", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	sessions, err := persist.List()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}
