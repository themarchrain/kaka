package redis

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	o := defaultOptions()
	if o.keyPrefix != "kaka:" {
		t.Errorf("default keyPrefix = %q, want %q", o.keyPrefix, "kaka:")
	}
	if o.keyTTL != 30*time.Minute {
		t.Errorf("default keyTTL = %v, want 30m", o.keyTTL)
	}
	if o.policy != ErrorFailClosed {
		t.Errorf("default policy = %v, want ErrorFailClosed", o.policy)
	}
	if o.onError != nil {
		t.Error("default onError = non-nil, want nil")
	}
}

func TestOptionsApply(t *testing.T) {
	o := defaultOptions()
	var gotErr error
	apply := []Option{
		WithKeyPrefix("app:"),
		WithKeyTTL(time.Minute),
		WithErrorPolicy(ErrorFailOpen),
		WithOnError(func(err error) { gotErr = err }),
	}
	for _, opt := range apply {
		opt(&o)
	}
	if o.keyPrefix != "app:" {
		t.Errorf("keyPrefix = %q, want %q", o.keyPrefix, "app:")
	}
	if o.keyTTL != time.Minute {
		t.Errorf("keyTTL = %v, want 1m", o.keyTTL)
	}
	if o.policy != ErrorFailOpen {
		t.Errorf("policy = %v, want ErrorFailOpen", o.policy)
	}
	if o.onError == nil {
		t.Fatal("onError = nil, want non-nil")
	}
	o.onError(errors.New("boom"))
	if gotErr == nil || gotErr.Error() != "boom" {
		t.Errorf("onError got %v, want boom", gotErr)
	}
}
