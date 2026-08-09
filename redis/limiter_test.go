package redis

import (
	"errors"
	"testing"
)

func TestFallbackFailClosed(t *testing.T) {
	l := newLimiter(nil)
	res := l.fallback(errors.New("boom"))
	if res.Allowed {
		t.Error("fail-closed fallback should deny")
	}
	if res.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", res.Remaining)
	}
}

func TestFallbackFailOpen(t *testing.T) {
	l := newLimiter(nil, WithErrorPolicy(ErrorFailOpen))
	res := l.fallback(errors.New("boom"))
	if !res.Allowed {
		t.Error("fail-open fallback should allow")
	}
}

func TestFallbackReportsError(t *testing.T) {
	var reported error
	l := newLimiter(nil, WithOnError(func(err error) { reported = err }))
	_ = l.fallback(errors.New("boom"))
	if reported == nil || reported.Error() != "boom" {
		t.Errorf("onError got %v, want boom", reported)
	}
}

func TestKeyFor(t *testing.T) {
	l := newLimiter(nil, WithKeyPrefix("app:"))
	if got := l.keyFor("user:1"); got != "app:user:1" {
		t.Errorf("keyFor = %q, want %q", got, "app:user:1")
	}
}
