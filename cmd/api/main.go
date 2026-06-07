package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/ritik-kharya/gobouncer/config"
	"github.com/ritik-kharya/gobouncer/internal/limiter"
)

func main() {
	cfg := config.Load()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("cannot connect to redis: %v", err)
	}
	log.Printf("Redis connected to %s", cfg.RedisAddr)

	var l limiter.Algorithm
	if cfg.Algorithm == "gcra" {
		l = limiter.NewGCRA(rdb)
	} else {
		l = limiter.NewSlidingWindow(rdb)
	}
	log.Printf("Algorithm: %s", cfg.Algorithm)

	http.HandleFunc("/check", makeCheckHandler(l))
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	log.Printf("Server running on port %s", cfg.ServerPort)
	if err := http.ListenAndServe(cfg.ServerPort, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}