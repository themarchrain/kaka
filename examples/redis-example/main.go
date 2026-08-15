// Command redis-example demonstrates Redis distributed rate limiting and
// layered (local pre-check + Redis authority) rate limiting with the
// net/http middleware. Requires Redis: REDIS_ADDR env (default
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
	"github.com/themarchrain/kaka/layered"
	"github.com/themarchrain/kaka/memory"
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

	// Pure Redis limiter: every decision is a distributed round trip.
	limiter := redislimiter.NewTokenBucket(client, 2, 1)

	// Layered limiter: the in-memory layer rejects floods locally, the
	// Redis layer stays the authoritative quota. The remote layer uses a
	// distinct key prefix so the demo routes keep independent buckets.
	layeredLimiter := layered.New(
		memory.NewTokenBucket(2, 1),
		redislimiter.NewTokenBucket(client, 2, 1, redislimiter.WithKeyPrefix("kaka:layered:")),
	)

	keyFunc := func(r *http.Request) string {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}

	testMux := http.NewServeMux()
	testMux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "success"})
	})

	layeredMux := http.NewServeMux()
	layeredMux.HandleFunc("GET /api/layered", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "layered success"})
	})

	root := http.NewServeMux()
	root.Handle("/api/test", httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: limiter,
		KeyFunc: keyFunc,
	})(testMux))
	root.Handle("/api/layered", httpmiddleware.Middleware(httpmiddleware.Config{
		Limiter: layeredLimiter,
		KeyFunc: keyFunc,
	})(layeredMux))

	log.Printf("redis example server listening on :8082 (redis at %s)", addr)
	log.Println("Test: curl http://localhost:8082/api/test")
	log.Println("Test: curl http://localhost:8082/api/layered")
	if err := http.ListenAndServe(":8082", root); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
