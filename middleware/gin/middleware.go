package gin

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/themarchrain/kaka"
)

// Config holds the Gin middleware configuration.
type Config struct {
	// Limiter is the rate limiter instance.
	Limiter kaka.Limiter

	// KeyFunc extracts the rate limit key from a Gin context.
	// It defaults to ClientIP().
	KeyFunc func(c *gin.Context) string

	// DeniedHandler handles requests that are denied.
	// It defaults to returning 429 Too Many Requests.
	DeniedHandler func(c *gin.Context)

	// ErrorHandler handles errors returned by the limiter.
	// The default is fail-open.
	ErrorHandler func(c *gin.Context, err error)

	// Headers controls whether X-RateLimit-* response headers are set.
	// Defaults to true.
	//
	// Deprecated: use DisableHeaders to turn headers off. This field is kept for
	// source compatibility and no longer controls the default header behavior.
	Headers bool

	// DisableHeaders disables the X-RateLimit-* response headers.
	// Defaults to false, meaning headers are written.
	DisableHeaders bool
}

// NewLimiterMiddleware creates a rate limiting middleware for Gin.
func NewLimiterMiddleware(config Config) gin.HandlerFunc {
	if config.Limiter == nil {
		panic("gin middleware: limiter must not be nil")
	}
	// 设置默认值
	if config.KeyFunc == nil {
		config.KeyFunc = func(c *gin.Context) string {
			return c.ClientIP()
		}
	}
	if config.DeniedHandler == nil {
		config.DeniedHandler = func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
		}
	}
	return func(c *gin.Context) {
		key := config.KeyFunc(c)

		result, err := config.Limiter.Allow(c.Request.Context(), key)
		if err != nil {
			if config.ErrorHandler != nil {
				config.ErrorHandler(c, err)
				return
			}
			// 限流器内部错误，放行请求（降级策略）
			c.Next()
			return
		}

		// 设置响应头
		if !config.DisableHeaders {
			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
			if !result.Allowed && result.RetryAfter > 0 {
				c.Header("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(result.RetryAfter)))
			}
		}

		if !result.Allowed {
			config.DeniedHandler(c)
			return
		}

		c.Next()
	}
}

func retryAfterSeconds(d time.Duration) int {
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}
