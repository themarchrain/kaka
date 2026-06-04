package gin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gonic "github.com/gin-gonic/gin"
	"github.com/themarchrain/kaka"
	ginmiddleware "github.com/themarchrain/kaka/middleware/gin"
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

func TestMiddlewareWritesHeadersByDefault(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed:   true,
			Remaining: 7,
		},
	}
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
	}))
	router.GET("/", func(c *gonic.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if got := recorder.Header().Get("X-RateLimit-Remaining"); got != "7" {
		t.Fatalf("expected remaining header 7, got %q", got)
	}
}

func TestMiddlewareUsesClientIPByDefault(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed: true,
		},
	}
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
	}))
	router.GET("/", func(c *gonic.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.1:12345"
	router.ServeHTTP(recorder, request)

	if limiter.key != "192.0.2.1" {
		t.Fatalf("expected default key from ClientIP, got %q", limiter.key)
	}
}

func TestMiddlewareCanDisableHeaders(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed:   true,
			Remaining: 7,
		},
	}
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter:        limiter,
		DisableHeaders: true,
	}))
	router.GET("/", func(c *gonic.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if got := recorder.Header().Get("X-RateLimit-Remaining"); got != "" {
		t.Fatalf("expected no remaining header, got %q", got)
	}
}

func TestMiddlewareWritesRetryAfterWhenDenied(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: 500 * time.Millisecond,
		},
	}
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
	}))
	router.GET("/", func(c *gonic.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, recorder.Code)
	}
	if got := recorder.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected retry-after header 1, got %q", got)
	}
}

func TestMiddlewareUsesCustomDeniedHandler(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed: false,
		},
	}
	deniedCalled := false
	nextCalled := false
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
		DeniedHandler: func(c *gonic.Context) {
			deniedCalled = true
			c.AbortWithStatus(http.StatusAccepted)
		},
	}))
	router.GET("/", func(c *gonic.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if !deniedCalled {
		t.Fatal("expected custom denied handler to be called")
	}
	if nextCalled {
		t.Fatal("expected custom denied handler to stop next handler")
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
}

func TestMiddlewareUsesCustomKeyFunc(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	limiter := &stubLimiter{
		result: kaka.Result{
			Allowed: true,
		},
	}
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
		KeyFunc: func(c *gonic.Context) string {
			return c.GetHeader("X-User-ID")
		},
	}))
	router.GET("/", func(c *gonic.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-User-ID", "user:42")
	router.ServeHTTP(recorder, request)

	if limiter.key != "user:42" {
		t.Fatalf("expected custom key user:42, got %q", limiter.key)
	}
}

func TestMiddlewareFailsOpenOnLimiterError(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	expectedErr := errors.New("limiter unavailable")
	limiter := &stubLimiter{
		err: expectedErr,
	}
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
	}))
	router.GET("/", func(c *gonic.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected fail-open status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestMiddlewareUsesCustomErrorHandler(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	expectedErr := errors.New("limiter unavailable")
	limiter := &stubLimiter{
		err: expectedErr,
	}
	var handledErr error
	nextCalled := false
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
		ErrorHandler: func(c *gonic.Context, err error) {
			handledErr = err
			c.AbortWithStatus(http.StatusServiceUnavailable)
		},
	}))
	router.GET("/", func(c *gonic.Context) {
		nextCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if !errors.Is(handledErr, expectedErr) {
		t.Fatalf("expected handler error %v, got %v", expectedErr, handledErr)
	}
	if nextCalled {
		t.Fatal("expected custom error handler to stop next handler")
	}
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}

func TestMiddlewareFailsOpenOnBlankKeyError(t *testing.T) {
	gonic.SetMode(gonic.TestMode)

	limiter := &stubLimiter{
		err: errors.New("invalid key"),
	}
	router := gonic.New()
	router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
		Limiter: limiter,
		KeyFunc: func(c *gonic.Context) string {
			return "   "
		},
	}))
	router.GET("/", func(c *gonic.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if limiter.key != "   " {
		t.Fatalf("expected blank key to be passed to limiter, got %q", limiter.key)
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected blank key limiter error to fail open with status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestMiddlewarePanicsWithoutLimiter(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic without limiter")
		}
	}()

	_ = ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{})
}
