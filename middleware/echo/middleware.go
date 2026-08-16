// Package echo provides an Echo middleware adapter for kaka limiters.
//
// The middleware converts an Echo request into a kaka.Limiter.Allow call
// keyed by the client IP (customizable) and responds natively on denial.
package echo

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/themarchrain/kaka"
)

// Config holds the Echo middleware configuration.
type Config struct {
	// Limiter is the rate limiter instance.
	Limiter kaka.Limiter

	// KeyFunc extracts the rate limit key from an Echo context.
	// It defaults to RealIP().
	KeyFunc func(c echo.Context) string

	// DeniedHandler handles requests that are denied.
	// It defaults to returning 429 Too Many Requests.
	DeniedHandler func(c echo.Context)

	// ErrorHandler handles errors returned by the limiter.
	// The default is fail-open.
	ErrorHandler func(c echo.Context, err error)

	// DisableHeaders disables the X-RateLimit-* response headers.
	// Defaults to false, meaning headers are written.
	DisableHeaders bool
}

// NewLimiterMiddleware creates a rate limiting middleware for Echo.
// It panics if config.Limiter is nil.
func NewLimiterMiddleware(config Config) echo.MiddlewareFunc {
	if config.Limiter == nil {
		panic("echo middleware: limiter must not be nil")
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(c echo.Context) string {
			return c.RealIP()
		}
	}
	if config.DeniedHandler == nil {
		config.DeniedHandler = func(c echo.Context) {
			_ = c.JSON(http.StatusTooManyRequests, echo.Map{"error": "too many requests"})
		}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := config.KeyFunc(c)

			result, err := config.Limiter.Allow(c.Request().Context(), key)
			if err != nil {
				if config.ErrorHandler != nil {
					config.ErrorHandler(c, err)
					return nil
				}
				// Fail-open: the limiter's internal error must not block the request.
				return next(c)
			}

			if !config.DisableHeaders {
				c.Response().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
				if !result.Allowed && result.RetryAfter > 0 {
					c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(result.RetryAfter)))
				}
			}

			if !result.Allowed {
				config.DeniedHandler(c)
				return nil
			}

			return next(c)
		}
	}
}

func retryAfterSeconds(d time.Duration) int {
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}
