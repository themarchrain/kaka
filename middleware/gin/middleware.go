package gin

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/themarchrain/kaka"
)

// Config Gin 中间件配置
type Config struct {
	// Limiter 限流器实例
	Limiter kaka.Limiter

	// KeyFunc 从 Gin Context 中提取限流 key 的函数
	// 默认使用 ClientIP()
	KeyFunc func(c *gin.Context) string

	// DeniedHandler 被限流时的自定义处理函数
	// 默认返回 429 Too Many Requests
	DeniedHandler func(c *gin.Context)

	// Headers 是否设置 X-RateLimit-* 响应头
	// 默认 true
	//
	// Deprecated: use DisableHeaders to turn headers off. This field is kept for
	// source compatibility and no longer controls the default header behavior.
	Headers bool

	// DisableHeaders 是否禁用 X-RateLimit-* 响应头
	// 默认 false，表示写入响应头
	DisableHeaders bool
}

// NewLimiterMiddleware 创建 Gin 限流中间件
func NewLimiterMiddleware(config Config) gin.HandlerFunc {
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
