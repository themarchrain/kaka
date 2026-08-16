package layered

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/themarchrain/kaka"
)

type stubLimiter struct {
	result kaka.Result
	err    error
	calls  int
}

func (s *stubLimiter) Allow(ctx context.Context, key string) (kaka.Result, error) {
	s.calls++
	return s.result, s.err
}

type recordingSink struct {
	mu       sync.Mutex
	allowed  map[string]int
	rejected map[string]int
	errs     map[string]int
}

func newRecordingSink() *recordingSink {
	return &recordingSink{allowed: map[string]int{}, rejected: map[string]int{}, errs: map[string]int{}}
}
func (s *recordingSink) OnAllowed(tier string, r kaka.Result) {
	s.mu.Lock()
	s.allowed[tier]++
	s.mu.Unlock()
}
func (s *recordingSink) OnRejected(tier string, r kaka.Result) {
	s.mu.Lock()
	s.rejected[tier]++
	s.mu.Unlock()
}
func (s *recordingSink) OnError(tier string, err error) {
	s.mu.Lock()
	s.errs[tier]++
	s.mu.Unlock()
}
func (s *recordingSink) SetKeys(tier string, n int) {}
func (s *recordingSink) OnEvict(tier string)        {}

func TestLayered_WithSink_SplitsTiers(t *testing.T) {
	ctx := context.Background()

	t.Run("local denies without consulting remote", func(t *testing.T) {
		sink := newRecordingSink()
		local := &stubLimiter{result: kaka.Result{Allowed: false}}
		remote := &stubLimiter{result: kaka.Result{Allowed: true}}
		l := New(local, remote, WithMetricSink(sink))
		if _, err := l.Allow(ctx, "k"); err != nil {
			t.Fatal(err)
		}
		if remote.calls != 0 {
			t.Fatalf("remote must not be consulted on local deny, got %d calls", remote.calls)
		}
		if sink.rejected[kaka.TierLocal] != 1 || len(sink.allowed) != 0 {
			t.Fatalf("expected 1 local reject, got %v", sink)
		}
	})

	t.Run("remote allows", func(t *testing.T) {
		sink := newRecordingSink()
		local := &stubLimiter{result: kaka.Result{Allowed: true}}
		remote := &stubLimiter{result: kaka.Result{Allowed: true}}
		l := New(local, remote, WithMetricSink(sink))
		if _, err := l.Allow(ctx, "k"); err != nil {
			t.Fatal(err)
		}
		if sink.allowed[kaka.TierRemote] != 1 {
			t.Fatalf("expected 1 remote allow, got %v", sink)
		}
	})

	t.Run("remote denies", func(t *testing.T) {
		sink := newRecordingSink()
		local := &stubLimiter{result: kaka.Result{Allowed: true}}
		remote := &stubLimiter{result: kaka.Result{Allowed: false}}
		l := New(local, remote, WithMetricSink(sink))
		if _, err := l.Allow(ctx, "k"); err != nil {
			t.Fatal(err)
		}
		if sink.rejected[kaka.TierRemote] != 1 {
			t.Fatalf("expected 1 remote reject, got %v", sink)
		}
	})

	t.Run("local error falls through but is counted", func(t *testing.T) {
		sink := newRecordingSink()
		localErr := errors.New("local full")
		local := &stubLimiter{err: localErr}
		remote := &stubLimiter{result: kaka.Result{Allowed: true}}
		l := New(local, remote, WithMetricSink(sink))
		if _, err := l.Allow(ctx, "k"); err != nil {
			t.Fatal(err)
		}
		if sink.errs[kaka.TierLocal] != 1 || sink.allowed[kaka.TierRemote] != 1 {
			t.Fatalf("expected local error + remote allow, got %v", sink)
		}
	})

	t.Run("remote error propagates and is counted", func(t *testing.T) {
		sink := newRecordingSink()
		local := &stubLimiter{result: kaka.Result{Allowed: true}}
		remoteErr := errors.New("redis down")
		remote := &stubLimiter{err: remoteErr}
		l := New(local, remote, WithMetricSink(sink))
		if _, err := l.Allow(ctx, "k"); !errors.Is(err, remoteErr) {
			t.Fatalf("expected remote error to propagate, got %v", err)
		}
		if sink.errs[kaka.TierRemote] != 1 {
			t.Fatalf("expected 1 remote error, got %v", sink)
		}
	})

	t.Run("blank key is a single-tier error", func(t *testing.T) {
		sink := newRecordingSink()
		local := &stubLimiter{result: kaka.Result{Allowed: true}}
		remote := &stubLimiter{result: kaka.Result{Allowed: true}}
		l := New(local, remote, WithMetricSink(sink))
		if _, err := l.Allow(ctx, "  "); err == nil {
			t.Fatal("expected ErrInvalidKey")
		}
		if local.calls != 0 || remote.calls != 0 {
			t.Fatal("layers must not be consulted for blank key")
		}
		if sink.errs[kaka.TierSingle] != 1 {
			t.Fatalf("expected 1 single-tier error, got %v", sink)
		}
	})
}
