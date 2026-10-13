package circuitbreaker

import (
	"errors"
	"testing"
)

func TestExecuteSuccess(t *testing.T) {
	v, err := Execute(DefaultBreaker, func() (int, error) {
		return 42, nil
	}, 0)
	if err != nil || v != 42 {
		t.Errorf("expected 42 nil, got %d %v", v, err)
	}
}

func TestExecuteFallback(t *testing.T) {
	v, err := Execute(DefaultBreaker, func() (int, error) {
		return 0, errors.New("boom")
	}, 7)
	if err == nil {
		t.Error("expected error")
	}
	if v != 7 {
		t.Errorf("expected fallback 7, got %d", v)
	}
}

func TestExternalAPIBreaker(t *testing.T) {
	v, err := Execute(ExternalAPIBreaker, func() (string, error) {
		return "ok", nil
	}, "fallback")
	if err != nil || v != "ok" {
		t.Errorf("expected ok, got %q %v", v, err)
	}
}
