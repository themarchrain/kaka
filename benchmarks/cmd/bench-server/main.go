// Command bench-server 启动一个 HTTP 压测目标服务。
// --impl kaka：Kaka httpmiddleware + memory TokenBucket(100, 10)
// --impl xtime：x/time/rate 全局 limiter 薄中间件
// --keymode fixed（默认）：Kaka 使用固定 key "global"，对齐 x/time/rate 全局单 limiter 语义
// --keymode remote：Kaka 使用 RemoteAddr 作为 key（按连接分 key）
// --keymode many：Kaka 按连接哈希分入 10000 个 key 池（多用户场景）
// --maxkeys / --keyttl：Kaka 生命周期治理参数（0 表示不启用）
// /metrics 端点返回内存/GC 采样，且不受限流中间件影响
package main

import (
	"context"
	"encoding/json"
	"flag"
	"hash/crc32"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka/benchmarks/compare"
	"github.com/themarchrain/kaka/memory"
	httpmiddleware "github.com/themarchrain/kaka/middleware/http"
	redislimiter "github.com/themarchrain/kaka/redis"
	"golang.org/x/time/rate"
)

func main() {
	impl := flag.String("impl", "kaka", "limiter implementation: kaka | xtime | redis")
	keymode := flag.String("keymode", "fixed", "kaka key mode: fixed | remote | many")
	maxkeys := flag.Uint("maxkeys", 0, "kaka max keys (0 = unlimited)")
	keyttl := flag.Duration("keyttl", 0, "kaka key ttl (0 = disabled)")
	addr := flag.String("addr", ":8081", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	})

	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"heapAlloc":   m.HeapAlloc,
			"heapObjects": m.HeapObjects,
			"numGC":       m.NumGC,
			"goroutines":  runtime.NumGoroutine(),
		})
	})

	var handler http.Handler
	switch *impl {
	case "kaka":
		var opts []memory.Option
		if *maxkeys > 0 {
			opts = append(opts, memory.WithMaxKeys(int(*maxkeys)))
		}
		if *keyttl > 0 {
			opts = append(opts, memory.WithKeyTTL(*keyttl))
		}
		limiter := memory.NewTokenBucket(100, 10, opts...)
		var keyFunc func(r *http.Request) string
		switch *keymode {
		case "fixed":
			keyFunc = func(r *http.Request) string { return "global" }
		case "remote":
			keyFunc = func(r *http.Request) string { return r.RemoteAddr }
		case "many":
			// 按连接哈希分入 10000 个 key 池（多用户场景）
			keyFunc = func(r *http.Request) string {
				return "user:" + strconv.Itoa(int(crc32.ChecksumIEEE([]byte(r.RemoteAddr)))%10000)
			}
		default:
			log.Fatalf("unknown keymode %q", *keymode)
		}
		handler = httpmiddleware.Middleware(httpmiddleware.Config{
			Limiter: limiter,
			KeyFunc: keyFunc,
		})(mux)
	case "xtime":
		handler = compare.RateLimitMiddleware(rate.NewLimiter(rate.Limit(10), 100), mux)
	case "redis":
		// Redis 分布式 TokenBucket：固定 key（对齐 kaka fixed 全局语义）
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "127.0.0.1:6379"
		}
		client := redis.NewClient(&redis.Options{Addr: redisAddr})
		if err := client.Ping(context.Background()).Err(); err != nil {
			log.Fatalf("redis at %s: %v", redisAddr, err)
		}
		limiter := redislimiter.NewTokenBucket(client, 100, 10)
		handler = httpmiddleware.Middleware(httpmiddleware.Config{
			Limiter: limiter,
			KeyFunc: func(r *http.Request) string { return "global" },
		})(mux)
	default:
		log.Fatalf("unknown impl %q", *impl)
	}

	// /metrics 不受限流中间件影响
	combined := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			metricsMux.ServeHTTP(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})

	log.Printf("bench-server (%s, keymode=%s, maxkeys=%d, keyttl=%s) listening on %s",
		*impl, *keymode, *maxkeys, *keyttl, *addr)
	if err := http.ListenAndServe(*addr, combined); err != nil {
		log.Fatal(err)
	}
}
