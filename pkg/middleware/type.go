package middleware

import (
	"net/http"
)

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

// config holds all middleware configuration
type config struct {
	client   *Client
	limit    int64
	windowMs int64
	keyFunc  KeyFunc
}
