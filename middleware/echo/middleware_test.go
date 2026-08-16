package echo_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/themarchrain/kaka"
	echomiddleware "github.com/themarchrain/kaka/middleware/echo"
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

func newTestRouter(limiter kaka.Limiter, config *echomiddleware.Config) *echo.Echo {
	e := echo.New()
	cfg := echomiddleware.Config{Limiter: limiter}
	if config != nil {
		cfg = *config
		if cfg.Limiter == nil {
			cfg.Limiter = limiter
		}
	}
	e.Use(echomiddleware.NewLimiterMiddleware(cfg))
	e.GET("/", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	return e
}

func doGet(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestMiddlewareWritesHeadersByDefault(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: true, Remaining: 7}}
	rec := doGet(newTestRouter(limiter, nil))
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "7" {
		t.Fatalf("expected X-RateLimit-Remaining=7, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddlewareWritesRetryAfterOnDeny(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: false, RetryAfter: 1500 * time.Millisecond}}
	rec := doGet(newTestRouter(limiter, nil))
	if got := rec.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After=2 (ceil), got %q", got)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}

func TestMiddlewareUsesCustomDeniedHandler(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: false}}
	called := false
	router := newTestRouter(limiter, &echomiddleware.Config{
		DeniedHandler: func(c echo.Context) {
			called = true
			c.String(http.StatusForbidden, "nope")
		},
	})
	rec := doGet(router)
	if !called {
		t.Fatal("expected custom denied handler to be called")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestMiddlewareUsesErrorHandler(t *testing.T) {
	limiter := &stubLimiter{err: errors.New("boom")}
	called := false
	router := newTestRouter(limiter, &echomiddleware.Config{
		ErrorHandler: func(c echo.Context, err error) {
			called = true
			c.String(http.StatusInternalServerError, "err")
		},
	})
	rec := doGet(router)
	if !called {
		t.Fatal("expected error handler to be called")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestMiddlewareFailOpenWithoutErrorHandler(t *testing.T) {
	limiter := &stubLimiter{err: errors.New("boom")}
	rec := doGet(newTestRouter(limiter, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected fail-open 200, got %d", rec.Code)
	}
}

func TestMiddlewareDisableHeaders(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: true, Remaining: 7}}
	rec := doGet(newTestRouter(limiter, &echomiddleware.Config{DisableHeaders: true}))
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "" {
		t.Fatalf("expected no X-RateLimit-Remaining header, got %q", got)
	}
}

func TestMiddlewareUsesCustomKeyFunc(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: true}}
	router := newTestRouter(limiter, &echomiddleware.Config{
		KeyFunc: func(c echo.Context) string { return "custom-key" },
	})
	doGet(router)
	if limiter.key != "custom-key" {
		t.Fatalf("expected limiter key=custom-key, got %q", limiter.key)
	}
}

func TestMiddlewarePanicsWithoutLimiter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic without limiter")
		}
	}()
	echomiddleware.NewLimiterMiddleware(echomiddleware.Config{})
}
