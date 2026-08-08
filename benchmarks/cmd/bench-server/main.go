// Command bench-server 启动一个 HTTP 压测目标服务。
// --impl kaka：Kaka httpmiddleware + memory TokenBucket(100, 10)（单 key）
// --impl xtime：x/time/rate 全局 limiter 薄中间件
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/themarchrain/kaka/benchmarks/compare"
	"github.com/themarchrain/kaka/memory"
	httpmiddleware "github.com/themarchrain/kaka/middleware/http"
	"golang.org/x/time/rate"
)

func main() {
	impl := flag.String("impl", "kaka", "limiter implementation: kaka | xtime")
	addr := flag.String("addr", ":8081", "listen address")
	flag.Parse()

	var handler http.Handler
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	})

	switch *impl {
	case "kaka":
		limiter := memory.NewTokenBucket(100, 10)
		handler = httpmiddleware.Middleware(httpmiddleware.Config{
			Limiter: limiter,
			KeyFunc: func(r *http.Request) string { return r.RemoteAddr },
		})(mux)
	case "xtime":
		handler = compare.RateLimitMiddleware(rate.NewLimiter(rate.Limit(10), 100), mux)
	default:
		log.Fatalf("unknown impl %q", *impl)
	}

	log.Printf("bench-server (%s) listening on %s", *impl, *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}
