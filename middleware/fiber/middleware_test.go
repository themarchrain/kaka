package fiber_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/themarchrain/kaka"
	fibermiddleware "github.com/themarchrain/kaka/middleware/fiber"
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

func newTestApp(limiter kaka.Limiter, config *fibermiddleware.Config) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	cfg := fibermiddleware.Config{Limiter: limiter}
	if config != nil {
		cfg = *config
		if cfg.Limiter == nil {
			cfg.Limiter = limiter
		}
	}
	app.Use(fibermiddleware.NewLimiterMiddleware(cfg))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func doGet(app *fiber.App) *http.Response {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func TestMiddlewareWritesHeadersByDefault(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: true, Remaining: 7}}
	resp := doGet(newTestApp(limiter, nil))
	defer resp.Body.Close()
	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "7" {
		t.Fatalf("expected X-RateLimit-Remaining=7, got %q", got)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMiddlewareWritesRetryAfterOnDeny(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: false, RetryAfter: 1500 * time.Millisecond}}
	resp := doGet(newTestApp(limiter, nil))
	defer resp.Body.Close()
	if got := resp.Header.Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After=2 (ceil), got %q", got)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
}

func TestMiddlewareUsesCustomDeniedHandler(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: false}}
	called := false
	app := newTestApp(limiter, &fibermiddleware.Config{
		DeniedHandler: func(c *fiber.Ctx) {
			called = true
			_ = c.Status(http.StatusForbidden).SendString("nope")
		},
	})
	resp := doGet(app)
	defer resp.Body.Close()
	if !called {
		t.Fatal("expected custom denied handler to be called")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestMiddlewareUsesErrorHandler(t *testing.T) {
	limiter := &stubLimiter{err: errors.New("boom")}
	called := false
	app := newTestApp(limiter, &fibermiddleware.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) {
			called = true
			_ = c.Status(http.StatusInternalServerError).SendString("err")
		},
	})
	resp := doGet(app)
	defer resp.Body.Close()
	if !called {
		t.Fatal("expected error handler to be called")
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestMiddlewareFailOpenWithoutErrorHandler(t *testing.T) {
	limiter := &stubLimiter{err: errors.New("boom")}
	resp := doGet(newTestApp(limiter, nil))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected fail-open 200, got %d", resp.StatusCode)
	}
}

func TestMiddlewareDisableHeaders(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: true, Remaining: 7}}
	resp := doGet(newTestApp(limiter, &fibermiddleware.Config{DisableHeaders: true}))
	defer resp.Body.Close()
	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "" {
		t.Fatalf("expected no X-RateLimit-Remaining header, got %q", got)
	}
}

func TestMiddlewareUsesCustomKeyFunc(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: true}}
	app := newTestApp(limiter, &fibermiddleware.Config{
		KeyFunc: func(c *fiber.Ctx) string { return "custom-key" },
	})
	resp := doGet(app)
	resp.Body.Close()
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
	fibermiddleware.NewLimiterMiddleware(fibermiddleware.Config{})
}

func TestMiddlewareUsesClientIPByDefault(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: true}}
	app := newTestApp(limiter, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	// 默认配置下 fiber IP() 不信任代理头，返回 socket 远端 IP（app.Test 下为 0.0.0.0）。
	// 断言"非空且不等于伪造的 XFF"即可验证默认路径真实走通。
	resp, _ := app.Test(req)
	resp.Body.Close()
	if limiter.key == "" || limiter.key == "203.0.113.9" {
		t.Fatalf("expected default socket IP (not XFF), got %q", limiter.key)
	}
}

func TestMiddlewareStopsChainAfterDeny(t *testing.T) {
	limiter := &stubLimiter{result: kaka.Result{Allowed: false}}
	nextCalled := false
	cfg := fibermiddleware.Config{Limiter: limiter}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(fibermiddleware.NewLimiterMiddleware(cfg))
	app.Get("/", func(c *fiber.Ctx) error {
		nextCalled = true
		return c.SendString("ok")
	})
	resp := doGet(app)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if nextCalled {
		t.Fatal("expected handler NOT to run after denial")
	}
}

func TestMiddlewareStopsChainAfterError(t *testing.T) {
	limiter := &stubLimiter{err: errors.New("boom")}
	nextCalled := false
	cfg := fibermiddleware.Config{Limiter: limiter}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(fibermiddleware.NewLimiterMiddleware(cfg))
	app.Get("/", func(c *fiber.Ctx) error {
		nextCalled = true
		return c.SendString("ok")
	})
	resp := doGet(app)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected fail-open 200, got %d", resp.StatusCode)
	}
	if !nextCalled {
		t.Fatal("expected handler to run on fail-open")
	}
}
