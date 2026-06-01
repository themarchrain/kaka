package httpmiddleware

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/themarchrain/kaka"
)

// Config HTTP 中间件配置
type Config struct {
	Limiter kaka.Limiter

	KeyFunc func(r *http.Request) string

	DeniedHandler func(w http.ResponseWriter, r *http.Request, result kaka.Result)

	ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

	DisableHeaders bool
}

// Middleware 创建标准库 HTTP 限流中间件
func Middleware(config Config) func(http.Handler) http.Handler {
	if config.Limiter == nil {
		panic("http middleware: limiter must not be nil")
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(r *http.Request) string {
			return r.RemoteAddr
		}
	}
	if config.DeniedHandler == nil {
		config.DeniedHandler = func(w http.ResponseWriter, r *http.Request, result kaka.Result) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "too many requests",
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := config.KeyFunc(r)

			result, err := config.Limiter.Allow(r.Context(), key)
			if err != nil {
				if config.ErrorHandler != nil {
					config.ErrorHandler(w, r, err)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if !config.DisableHeaders {
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
				if !result.Allowed && result.RetryAfter > 0 {
					w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds(result.RetryAfter)))
				}
			}

			if !result.Allowed {
				config.DeniedHandler(w, r, result)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func retryAfterSeconds(d time.Duration) int {
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}
