package core

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ShutdownState tracks the graceful shutdown lifecycle.
type ShutdownState struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	saved    bool
	shutdown bool
}

// NewShutdown creates a ShutdownState with a cancelable context.
func NewShutdown() *ShutdownState {
	ctx, cancel := context.WithCancel(context.Background())
	return &ShutdownState{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

// Ctx returns the shutdown-aware context.
func (s *ShutdownState) Ctx() context.Context { return s.ctx }

// Done returns a channel that closes when shutdown completes.
func (s *ShutdownState) Done() <-chan struct{} { return s.done }

// MarkSaved records that the session was saved.
func (s *ShutdownState) MarkSaved() {
	s.mu.Lock()
	s.saved = true
	s.mu.Unlock()
}

// Shutdown initiates graceful shutdown.
func (s *ShutdownState) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shutdown {
		return
	}
	s.shutdown = true
	s.cancel()
	// Give 10 seconds for cleanup, then force close
	go func() {
		select {
		case <-time.After(10 * time.Second):
			Warn("shutdown: timed out, forcing exit")
		case <-s.done:
		}
		close(s.done)
	}()
}

// SignalHandler blocks until SIGINT/SIGTERM, then initiates shutdown.
// Second signal → immediate os.Exit(1).
func SignalHandler(s *ShutdownState) {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	Info("shutdown: signal received")
	s.Shutdown()
	go func() {
		<-sig // second signal
		Warn("shutdown: second signal, forcing exit")
		os.Exit(1)
	}()
	signal.Stop(sig)
}
