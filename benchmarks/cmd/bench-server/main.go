// Command bench-server 启动一个 HTTP 压测目标服务。
// --impl kaka：Kaka httpmiddleware + memory TokenBucket(100, 10)
// --impl xtime：x/time/rate 全局 limiter 薄中间件
// --keymode fixed（默认）：Kaka 使用固定 key "global"，对齐 x/time/rate 全局单 limiter 语义
// --keymode remote：Kaka 使用 RemoteAddr 作为 key（按连接分 key）
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
	keymode := flag.String("keymode", "fixed", "kaka key mode: fixed | remote")
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
		var keyFunc func(r *http.Request) string
		switch *keymode {
		case "fixed":
			keyFunc = func(r *http.Request) string { return "global" }
		case "remote":
			keyFunc = func(r *http.Request) string { return r.RemoteAddr }
		default:
			log.Fatalf("unknown keymode %q", *keymode)
		}
		handler = httpmiddleware.Middleware(httpmiddleware.Config{
			Limiter: limiter,
			KeyFunc: keyFunc,
		})(mux)
	case "xtime":
		handler = compare.RateLimitMiddleware(rate.NewLimiter(rate.Limit(10), 100), mux)
	default:
		log.Fatalf("unknown impl %q", *impl)
	}

	log.Printf("bench-server (%s, keymode=%s) listening on %s", *impl, *keymode, *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}
