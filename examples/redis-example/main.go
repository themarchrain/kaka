// Command redis-example demonstrates Redis distributed rate limiting with
// the net/http middleware. Requires Redis: REDIS_ADDR env (default
// 127.0.0.1:6379).
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
	httpmiddleware "github.com/themarchrain/kaka/middleware/http"
	redislimiter "github.com/themarchrain/kaka/redis"
)

func main() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("cannot connect to redis at %s: %v", addr, err)
	}

	limiter := redislimiter.NewTokenBucket(client, 2, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
	})

	handler := httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
		KeyFunc: func(r *http.Request) string {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return r.RemoteAddr
			}
			return host
		},
	})(mux)

	log.Printf("redis example server listening on :8082 (redis at %s)", addr)
	log.Println("Test: curl http://localhost:8082/api/test")
	if err := http.ListenAndServe(":8082", handler); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
