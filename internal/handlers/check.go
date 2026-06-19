package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ritik-kharya/gobouncer/internal/limiter"
)

// NewCheckHandler returns a handler that checks the rate limit using the selected algorithm.
func NewCheckHandler(algos Algorithms) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("failed to decode check request", "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Key == "" {
			http.Error(w, "Missing key", http.StatusBadRequest)
			return
		}
		if req.Limit <= 0 {
			http.Error(w, "Invalid limit", http.StatusBadRequest)
			return
		}
		if req.WindowMs <= 0 {
			http.Error(w, "Invalid window_ms", http.StatusBadRequest)
			return
		}

		var algo limiter.Algorithm
		switch req.Algorithm {
		case "gcra":
			algo = algos.GCRA
		case "sliding_window", "":
			algo = algos.SlidingWindow
		default:
			slog.Warn("unknown algorithm requested, falling back to sliding window", "algo", req.Algorithm)
			algo = algos.SlidingWindow
		}

		if algo == nil {
			slog.Error("requested algorithm is not initialized", "algo", req.Algorithm)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		result := algo.Check(r.Context(), req.Key, req.Limit, req.WindowMs)

		// Set standard rate limit headers
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(req.Limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))

		if result.RetryAfter > 0 {
			retrySeconds := result.RetryAfter / 1000
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(retrySeconds, 10))
			w.WriteHeader(http.StatusTooManyRequests)
		}

		if err := json.NewEncoder(w).Encode(result); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}
