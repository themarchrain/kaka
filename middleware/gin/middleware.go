package gin

import (
	"fmt"
	"net/http"

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
	Headers bool
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
	if !config.Headers {
		config.Headers = true
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
		if config.Headers {
			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
			if !result.Allowed {
				c.Header("Retry-After", fmt.Sprintf("%d", int(result.RetryAfter.Seconds())))
			}
		}

		if !result.Allowed {
			config.DeniedHandler(c)
			return
		}

		c.Next()
	}
}
