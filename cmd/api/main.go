package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/ritik-kharya/gobouncer/config"
	"github.com/ritik-kharya/gobouncer/internal/limiter"
)

type CheckRequest struct {
	Key 			string `json:"key" validate:"required"`
	Window 			int64 `json:"window" validate:"required"`
	Limit 			int64 `json:"limit" validate:"required"`
}

func main (){
	cfg := config.Load()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	l := limiter.New(rdb)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("cannot connect to redis: %v", err)
	}
	log.Printf("Redis connected to %s", cfg.RedisAddr)

	http.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		result := l.Check(req.Key, req.Window, req.Limit)

		w.Header().Set("content-type", "application/json")
		if result.RetryAfter > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", result.RetryAfter))
		}
		json.NewEncoder(w).Encode(result)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// w.Write([]byte("OK"))
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	log.Println("Starting server...")
	log.Printf("Configuration loaded: %v", cfg)
	log.Println("Checking Redis connection...")
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	log.Println("Redis connection successful")	
	log.Printf("Server running on port %s", cfg.ServerPort)


	if err := http.ListenAndServe(cfg.ServerPort, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}	
	
}