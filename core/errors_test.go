package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorWrapAndCause(t *testing.T) {
	orig := fmt.Errorf("connection refused")
	wrapped := Transient("http.Do", orig)

	if !IsKind(wrapped, KindTransient) {
		t.Error("expected KindTransient")
	}
	if Cause(wrapped) != orig {
		t.Errorf("expected cause %v, got %v", orig, Cause(wrapped))
	}
}

func TestErrorIsKind(t *testing.T) {
	transient := Transient("api.Call", fmt.Errorf("timeout"))
	permanent := Permanent("handler.Process", transient)

	if !IsKind(permanent, KindTransient) {
		t.Error("expected KindTransient through chain")
	}
	if !IsKind(permanent, KindPermanent) {
		t.Error("expected KindPermanent at top level")
	}
}

func TestErrorErrorsIs(t *testing.T) {
	orig := fmt.Errorf("specific error")
	wrapped := NewError("op", KindPermanent, orig)

	if !errors.Is(wrapped, orig) {
		t.Error("expected errors.Is to find original via Unwrap")
	}
}

func TestErrorFormat(t *testing.T) {
	err := NewError("provider.Stream", KindTransient, fmt.Errorf("429 Too Many Requests"))
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestErrorKindOf(t *testing.T) {
	if KindOf(fmt.Errorf("plain")) != KindPermanent {
		t.Error("expected KindPermanent for plain error")
	}
	tr := Transient("op", fmt.Errorf("timeout"))
	if KindOf(tr) != KindTransient {
		t.Error("expected KindTransient")
	}
}

func TestUsageError(t *testing.T) {
	err := UsageError("validate", fmt.Errorf("invalid input"))
	if KindOf(err) != KindUsage {
		t.Error("expected KindUsage")
	}
}
