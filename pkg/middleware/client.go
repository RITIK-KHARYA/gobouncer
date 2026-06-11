// pkg/middleware/client.go
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Result mirrors internal/limiter.Result
// duplicated here so pkg/ has zero dependency on internal/
type Result struct {
	Allowed    bool  `json:"allowed"`
	Remaining  int   `json:"remaining"`
	RetryAfter int64 `json:"retry_after,omitempty"`
}

// checkRequest is what we POST to /check
type checkRequest struct {
	Key      string `json:"key"`
	Limit    int64  `json:"limit"`
	WindowMs int64  `json:"window_ms"`
}

// Client is the GoBouncer HTTP client.
// Create once at startup, reuse everywhere.
// It is safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
	failOpen   bool
}

// Option is the functional options type
type Option func(*Client)

// WithTimeout sets the max time to wait for GoBouncer to respond.
// Default: 150ms. Keep this tight — a slow rate limiter is worse than none.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithFailOpen controls what happens if GoBouncer is unreachable.
// true  = allow the request through (default — availability over security)
// false = deny the request (security over availability)
func WithFailOpen(open bool) Option {
	return func(c *Client) {
		c.failOpen = open
	}
}

// NewClient creates a GoBouncer HTTP client.
// baseURL is the address of your running GoBouncer service e.g. "http://localhost:8080"
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:  baseURL,
		failOpen: true, // safe default
		httpClient: &http.Client{
			Timeout: 150 * time.Millisecond,
			// reuse TCP connections — critical for performance
			// net/http.DefaultTransport already does this,
			// but we set it explicitly so nobody accidentally replaces it
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}

	// apply any options the caller passed
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Check asks GoBouncer: should this key be allowed right now?
// ctx carries the deadline from the incoming request — if the user
// cancelled their request, we cancel the GoBouncer call too.
func (c *Client) Check(ctx context.Context, key string, limit, windowMs int64) (Result, error) {
	// build the request body
	body, err := json.Marshal(checkRequest{
		Key:      key,
		Limit:    limit,
		WindowMs: windowMs,
	})
	if err != nil {
		return c.onError(fmt.Errorf("gobouncer: marshal error: %w", err))
	}

	// build the HTTP request — attach ctx so cancellation propagates
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/check",
		bytes.NewReader(body),
	)
	if err != nil {
		return c.onError(fmt.Errorf("gobouncer: build request error: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")

	// fire the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.onError(fmt.Errorf("gobouncer: request failed: %w", err))
	}

	// run this function at last -- since using defer function
	defer resp.Body.Close()

	// parse the response
	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return c.onError(fmt.Errorf("gobouncer: decode error: %w", err))
	}

	return result, nil
}

// onError is called whenever the GoBouncer service is unreachable or broken.
// failOpen = true  → allow the request, return no error to the middleware
// failOpen = false → deny the request, caller gets the error
func (c *Client) onError(err error) (Result, error) {
	if c.failOpen {
		// GoBouncer is down — let traffic through, log the error upstream
		return Result{Allowed: true, Remaining: -1}, nil
	}
	return Result{Allowed: false}, err
}