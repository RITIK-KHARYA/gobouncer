package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ritik-kharya/gobouncer/internal/limiter"
)

// NewCheckHandler returns a handler that checks the rate limit using the selected algorithm.
func NewCheckHandler(algos Algorithms, policies PolicyStore) http.HandlerFunc {
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
		limit := req.Limit
		windowMs := req.WindowMs
		algorithm := req.Algorithm

		if req.Policy != "" {
			p, ok := policies.Get(req.Policy)
			if !ok {
				http.Error(w, "Unknown policy", http.StatusNotFound)
				return
			}
			limit = p.Limit
			windowMs = p.WindowMs
			algorithm = p.Algorithm
		}

		if limit <= 0 {
			http.Error(w, "Invalid limit", http.StatusBadRequest)
			return
		}
		if windowMs <= 0 {
			http.Error(w, "Invalid window_ms", http.StatusBadRequest)
			return
		}

		var algo limiter.Algorithm
		switch algorithm {
		case "gcra":
			algo = algos.GCRA
		case "sliding_window", "":
			algo = algos.SlidingWindow
		default:
			slog.Warn("unknown algorithm requested, falling back to sliding window", "algo", algorithm)
			algo = algos.SlidingWindow
		}

		if algo == nil {
			slog.Error("requested algorithm is not initialized", "algo", algorithm)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		result := algo.Check(r.Context(), req.Key, limit, windowMs)

		// Set standard rate limit headers
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		if req.Policy != "" {
			w.Header().Set("X-RateLimit-Policy", req.Policy)
		}

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
