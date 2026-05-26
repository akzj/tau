package core

import (
	"testing"
	"time"
)

func TestShutdownContextCancel(t *testing.T) {
	s := NewShutdown()
	s.Shutdown()
	select {
	case <-s.Ctx().Done():
		// expected
	case <-time.After(time.Second):
		t.Error("context not cancelled after shutdown")
	}
}

func TestShutdownMarkSaved(t *testing.T) {
	s := NewShutdown()
	if s.saved {
		t.Error("should not be saved initially")
	}
	s.MarkSaved()
	if !s.saved {
		t.Error("should be saved after MarkSaved")
	}
}

func TestShutdownDoubleSignal(t *testing.T) {
	s := NewShutdown()
	s.Shutdown()
	firstCancel := s.ctx.Err()
	s.Shutdown() // second call is no-op
	if s.ctx.Err() != firstCancel {
		t.Error("context should remain cancelled after second Shutdown")
	}
}
