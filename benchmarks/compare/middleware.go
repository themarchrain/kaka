package compare

import (
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimitMiddleware 用 x/time/rate 实现全局单 limiter 的 http 中间件，
// 用于端到端压测对比。语义差异：无 per-key 概念；kaka 侧用单 key 对齐。
func RateLimitMiddleware(l *rate.Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
