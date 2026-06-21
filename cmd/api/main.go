package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ritik-kharya/gobouncer/config"
	"github.com/ritik-kharya/gobouncer/internal/handlers"
	"github.com/ritik-kharya/gobouncer/internal/limiter"
	"github.com/ritik-kharya/gobouncer/internal/policy"
)

func main() {
	// Structured logger — JSON in prod, text for local dev
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("cannot connect to redis", "addr", cfg.RedisAddr, "error", err)
		os.Exit(1)
	}
	slog.Info("redis connected", "addr", cfg.RedisAddr)
	algos := handlers.Algorithms{
		SlidingWindow: limiter.NewSlidingWindow(rdb),
		GCRA:          limiter.NewGCRA(rdb, limiter.WithGCRAFailOpen(cfg.FailOpen)),
	}
	slog.Info("algorithms ready", "default", "sliding_window")

	policyStore := policy.DefaultStore()
	if cfg.PolicyFile != "" {
		loadedStore, err := policy.LoadFile(cfg.PolicyFile)
		if err != nil {
			slog.Error("cannot load policy file", "path", cfg.PolicyFile, "error", err)
			os.Exit(1)
		}
		policyStore = loadedStore
	}
	slog.Info("policies loaded", "count", len(policyStore.List()))

	mux := http.NewServeMux()
	mux.HandleFunc("/check", handlers.NewCheckHandler(algos, policyStore))
	mux.HandleFunc("/policies", handlers.NewPoliciesHandler(policyStore))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	srv := &http.Server{
		Addr:         cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("server starting", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown — wait for SIGINT or SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig)

	// Give in-flight requests 10 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
