package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ritik-kharya/gobouncer/internal/limiter"
)

type CheckRequest struct {
	Key    string `json:"key" validate:"required"`
	Window int64  `json:"window" validate:"required"`
	Limit  int64  `json:"limit" validate:"required"`
}

func makeCheckHandler(algo limiter.Algorithm) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		result := algo.Check(r.Context(), req.Key, req.Limit, req.Window)

		// Set standard rate limit headers
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(req.Limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))

		if result.RetryAfter > 0 {
			retrySeconds := result.RetryAfter / 1000
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySeconds))
			w.WriteHeader(http.StatusTooManyRequests)
		}

		json.NewEncoder(w).Encode(result)
	}
}
