package httpmiddleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
	httpmiddleware "github.com/themarchrain/kaka/middleware/http"
)

type stubLimiter struct {
	result kaka.Result
	err    error
	key    string
}

func (s *stubLimiter) Allow(ctx context.Context, key string) (kaka.Result, error) {
	s.key = key
	return s.result, s.err
}

func TestMiddlewareAllowsAndWritesHeadersByDefault(t *testing.T) {
	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed:   true,
			Remaining: 7,
		},
	}
	nextCalled := false
	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.1:12345"
	handler.ServeHTTP(recorder, request)

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if limiter.key != "192.0.2.1:12345" {
		t.Fatalf("expected default key from RemoteAddr, got %q", limiter.key)
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if got := recorder.Header().Get("X-RateLimit-Remaining"); got != "7" {
		t.Fatalf("expected remaining header 7, got %q", got)
	}
}

func TestMiddlewareCanDisableHeaders(t *testing.T) {
	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed:   true,
			Remaining: 7,
		},
	}
	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter:        limiter,
		DisableHeaders: true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if got := recorder.Header().Get("X-RateLimit-Remaining"); got != "" {
		t.Fatalf("expected no remaining header, got %q", got)
	}
}

func TestMiddlewareDeniesWithDefaultHandlerAndRetryAfter(t *testing.T) {
	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: 500 * time.Millisecond,
		},
	}
	nextCalled := false
	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, request)

	if nextCalled {
		t.Fatal("expected next handler not to be called")
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, recorder.Code)
	}
	if got := recorder.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected retry-after header 1, got %q", got)
	}
}

func TestMiddlewareUsesCustomKeyFunc(t *testing.T) {
	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed: true,
		},
	}
	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-User-ID")
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-User-ID", "user:42")
	handler.ServeHTTP(recorder, request)

	if limiter.key != "user:42" {
		t.Fatalf("expected custom key user:42, got %q", limiter.key)
	}
}

func TestMiddlewareUsesCustomDeniedHandler(t *testing.T) {
	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed:   false,
			Remaining: 3,
		},
	}
	var handledResult kaka.Result
	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
		DeniedHandler: func(w http.ResponseWriter, r *http.Request, result kaka.Result) {
			handledResult = result
			w.WriteHeader(http.StatusAccepted)
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, request)

	if handledResult.Remaining != 3 {
		t.Fatalf("expected handler result remaining 3, got %d", handledResult.Remaining)
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
}

func TestMiddlewareFailsOpenOnLimiterError(t *testing.T) {
	limiter := &stubLimiter{
		err: errors.New("limiter unavailable"),
	}
	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected fail-open status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestMiddlewareUsesCustomErrorHandler(t *testing.T) {
	expectedErr := errors.New("limiter unavailable")
	limiter := &stubLimiter{
		err: expectedErr,
	}
	var handledErr error
	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			handledErr = err
			w.WriteHeader(http.StatusServiceUnavailable)
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, request)

	if !errors.Is(handledErr, expectedErr) {
		t.Fatalf("expected handler error %v, got %v", expectedErr, handledErr)
	}
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}

func TestMiddlewarePanicsWithoutLimiter(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic without limiter")
		}
	}()

	_ = httpmiddleware.Middleware(httpmiddleware.Config{})
}
