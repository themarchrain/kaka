// Package fiber provides a Fiber middleware adapter for kaka limiters.
//
// The middleware converts a Fiber request into a kaka.Limiter.Allow call
// keyed by the client IP (customizable) and responds natively on denial.
package fiber

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/themarchrain/kaka"
)

// Config holds the Fiber middleware configuration.
type Config struct {
	// Limiter is the rate limiter instance.
	Limiter kaka.Limiter

	// KeyFunc extracts the rate limit key from a Fiber context.
	// It defaults to IP().
	KeyFunc func(c *fiber.Ctx) string

	// DeniedHandler handles requests that are denied.
	// It defaults to returning 429 Too Many Requests.
	DeniedHandler func(c *fiber.Ctx)

	// ErrorHandler handles errors returned by the limiter.
	// The default is fail-open.
	ErrorHandler func(c *fiber.Ctx, err error)

	// DisableHeaders disables the X-RateLimit-* response headers.
	// Defaults to false, meaning headers are written.
	DisableHeaders bool
}

// NewLimiterMiddleware creates a rate limiting middleware for Fiber.
// It panics if config.Limiter is nil.
func NewLimiterMiddleware(config Config) fiber.Handler {
	if config.Limiter == nil {
		panic("fiber middleware: limiter must not be nil")
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(c *fiber.Ctx) string {
			return c.IP()
		}
	}
	if config.DeniedHandler == nil {
		config.DeniedHandler = func(c *fiber.Ctx) {
			_ = c.Status(http.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests"})
		}
	}
	return func(c *fiber.Ctx) error {
		key := config.KeyFunc(c)

		result, err := config.Limiter.Allow(c.Context(), key)
		if err != nil {
			if config.ErrorHandler != nil {
				config.ErrorHandler(c, err)
				return nil
			}
			// Fail-open: the limiter's internal error must not block the request.
			return c.Next()
		}

		if !config.DisableHeaders {
			c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
			if !result.Allowed && result.RetryAfter > 0 {
				c.Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(result.RetryAfter)))
			}
		}

		if !result.Allowed {
			config.DeniedHandler(c)
			return nil
		}

		return c.Next()
	}
}

func retryAfterSeconds(d time.Duration) int {
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}
