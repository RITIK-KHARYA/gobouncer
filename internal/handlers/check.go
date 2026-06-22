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

		if len(req.Checks) > 0 {
			handleMultiCheck(w, r, algos, policies, req.Checks)
			return
		}

		check := Check{
			Key:       req.Key,
			Policy:    req.Policy,
			Limit:     req.Limit,
			WindowMs:  req.WindowMs,
			Algorithm: req.Algorithm,
		}
		resolved, result, ok := runCheck(w, r, algos, policies, check)
		if !ok {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(resolved.Limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
		if resolved.Policy != "" {
			w.Header().Set("X-RateLimit-Policy", resolved.Policy)
		}
		if result.RetryAfter > 0 {
			w.Header().Set("Retry-After", retryAfterSeconds(result.RetryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
		}

		if err := json.NewEncoder(w).Encode(result); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func  handleMultiCheck(w http.ResponseWriter, r *http.Request, algos Algorithms, policies PolicyStore, checks []Check) {
	response := MultiCheckResult{
		Allowed:   true,
		Remaining: -1,
		Checks:    make([]CheckResult, 0, len(checks)),
	}

	for _, check := range checks {
		resolved, result, ok := runCheck(w, r, algos, policies, check)
		if !ok {
			return
		}

		response.Checks = append(response.Checks, CheckResult{
			Name:       resolved.Name,
			Key:        resolved.Key,
			Policy:     resolved.Policy,
			Allowed:    result.Allowed,
			Remaining:  result.Remaining,
			RetryAfter: result.RetryAfter,
		})

		if response.Remaining < 0 || result.Remaining < response.Remaining {
			response.Remaining = result.Remaining
		}
		if !result.Allowed {
			response.Allowed = false
			if result.RetryAfter > response.RetryAfter {
				response.RetryAfter = result.RetryAfter
			}
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(response.Remaining, 10))
	if response.RetryAfter > 0 {
		w.Header().Set("Retry-After", retryAfterSeconds(response.RetryAfter))
		w.WriteHeader(http.StatusTooManyRequests)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode multi-check response", "error", err)
	}
}

func runCheck(w http.ResponseWriter, r *http.Request, algos Algorithms, policies PolicyStore, check Check) (Check, limiter.Result, bool) {
	if check.Key == "" {
		http.Error(w, "Missing key", http.StatusBadRequest)
		return Check{}, limiter.Result{}, false
	}

	resolved := check
	if resolved.Policy != "" {
		p, ok := policies.Get(resolved.Policy)
		if !ok {
			http.Error(w, "Unknown policy", http.StatusNotFound)
			return Check{}, limiter.Result{}, false
		}
		resolved.Limit = p.Limit
		resolved.WindowMs = p.WindowMs
		resolved.Algorithm = p.Algorithm
	}

	if resolved.Limit <= 0 {
		http.Error(w, "Invalid limit", http.StatusBadRequest)
		return Check{}, limiter.Result{}, false
	}
	if resolved.WindowMs <= 0 {
		http.Error(w, "Invalid window_ms", http.StatusBadRequest)
		return Check{}, limiter.Result{}, false
	}

	algo := selectAlgorithm(algos, resolved.Algorithm)
	if algo == nil {
		slog.Error("requested algorithm is not initialized", "algo", resolved.Algorithm)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return Check{}, limiter.Result{}, false
	}

	return resolved, algo.Check(r.Context(), resolved.Key, resolved.Limit, resolved.WindowMs), true
}

func selectAlgorithm(algos Algorithms, algorithm string) limiter.Algorithm {
	switch algorithm {
	case "gcra":
		return algos.GCRA
	case "sliding_window", "":
		return algos.SlidingWindow
	default:
		slog.Warn("unknown algorithm requested, falling back to sliding window", "algo", algorithm)
		return algos.SlidingWindow
	}
}

func retryAfterSeconds(ms int64) string {
	seconds := ms / 1000
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}
