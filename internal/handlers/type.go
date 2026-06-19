package handlers

import "github.com/ritik-kharya/gobouncer/internal/limiter"

// CheckRequest is what the caller sends
type CheckRequest struct {
	Key       string `json:"key"`
	Limit     int64  `json:"limit"`
	WindowMs  int64  `json:"window_ms"`
	Algorithm string `json:"algorithm"` // "sliding_window" or "gcra" — optional
}

// Algorithms holds both algorithm instances
// Handler picks which one to use based on the request
type Algorithms struct {
	SlidingWindow limiter.Algorithm
	GCRA          limiter.Algorithm
}
