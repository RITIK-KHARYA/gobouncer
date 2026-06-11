// pkg/middleware/middleware.go
package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// KeyFunc extracts a unique key from the request.
// This key identifies who is being rate limited.
// e.g. "ip:1.2.3.4", "user:123", "apikey:abc"
type KeyFunc func(r *http.Request) string

// config holds all middleware configuration
type config struct {
	client   *Client
	limit    int64
	windowMs int64
	keyFunc  KeyFunc
}

// MiddlewareOption configures the middleware
// separate from ClientOption so both are clean
type MiddlewareOption func(*config)

// WithLimit sets max requests allowed in the window
// e.g. gobouncer.WithLimit(100)
func WithLimit(n int64) MiddlewareOption {
	return func(c *config) {
		c.limit = n
	}
}

// WithWindow sets the time window
// e.g. gobouncer.WithWindow(time.Minute)
func WithWindow(d time.Duration) MiddlewareOption {
	return func(c *config) {
		c.windowMs = d.Milliseconds()
	}
}

// WithKeyFunc sets how the middleware identifies each caller.
// Default is IPKey if not provided.
func WithKeyFunc(fn KeyFunc) MiddlewareOption {
	return func(c *config) {
		c.keyFunc = fn
	}
}

// ── Built-in KeyFunc helpers ──────────────────────────────────────

// IPKey extracts the client IP from the request.
// Handles X-Forwarded-For for requests behind a proxy/load balancer.
var IPKey KeyFunc = func(r *http.Request) string {
	// check X-Forwarded-For first — set by proxies and load balancers
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return "ip:" + ip
	}
	// fall back to direct connection IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "ip:" + r.RemoteAddr
	}
	return "ip:" + ip
}

// HeaderKey returns a KeyFunc that uses a specific header value as the key.
// e.g. gobouncer.WithKeyFunc(gobouncer.HeaderKey("X-API-Key"))
func HeaderKey(header string) KeyFunc {
	return func(r *http.Request) string {
		val := r.Header.Get(header)
		if val == "" {
			// fall back to IP if header is missing
			return IPKey(r)
		}
		return header + ":" + val
	}
}

func RateLimit(client *Client, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	// build config with sensible defaults
	cfg := &config{
		client:   client,
		limit:    100,                        // 100 requests
		windowMs: time.Minute.Milliseconds(), // per minute
		keyFunc:  IPKey,                      // by IP unless overridden
	}

	// apply caller options
	for _, opt := range opts {
		opt(cfg)
	}

	// return the actual middleware function
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Step 1 — extract the key for this request
			key := cfg.keyFunc(r)

			// Step 2 — ask GoBouncer
			result, err := cfg.client.Check(r.Context(), key, cfg.limit, cfg.windowMs)
			if err != nil {
				// client.onError already decided fail-open/closed
				// if we get an error here it means failOpen=false and GoBouncer is down
				http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
				return
			}

			// Step 3 — set standard rate limit headers on EVERY response
			// even denied ones — clients need these to implement backoff
			setHeaders(w, cfg.limit, result)

			// Step 4 — allow or deny
			if !result.Allowed {
				writeDenied(w, result)
				return
			}

			// Step 5 — allowed, pass to the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// ── Header helpers ────────────────────────────────────────────────

// setHeaders writes standard rate limit headers.
// These are the headers every major API uses (GitHub, Stripe, etc.)
func setHeaders(w http.ResponseWriter, limit int64, result Result) {
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))

	if result.RetryAfter > 0 {
		// Retry-After is in seconds (HTTP spec)
		retrySeconds := result.RetryAfter / 1000
		if retrySeconds < 1 {
			retrySeconds = 1 // minimum 1 second
		}
		w.Header().Set("Retry-After", strconv.FormatInt(retrySeconds, 10))
	}
}

// writeDenied writes a 429 response with a JSON body
func writeDenied(w http.ResponseWriter, result Result) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests) // 429

	json.NewEncoder(w).Encode(map[string]any{
		"error":          "too many requests",
		"retry_after_ms": result.RetryAfter,
		"message":        fmt.Sprintf("slow down. retry after %dms", result.RetryAfter),
	})
}
