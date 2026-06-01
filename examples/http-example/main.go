package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"

	"github.com/themarchrain/kaka/memory"
	httpmiddleware "github.com/themarchrain/kaka/middleware/http"
)

func main() {
	limiter := memory.NewTokenBucket(2, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/test", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "success",
		})
	})
	mux.HandleFunc("GET /api/user/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"user_id": r.PathValue("id"),
			"message": "user info",
		})
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

	log.Println("HTTP example server starting on :8081...")
	log.Println("Test: curl http://localhost:8081/api/test")
	if err := http.ListenAndServe(":8081", handler); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
