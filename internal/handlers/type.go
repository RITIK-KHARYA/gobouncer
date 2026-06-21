package handlers

import (
	"github.com/ritik-kharya/gobouncer/internal/limiter"
	"github.com/ritik-kharya/gobouncer/internal/policy"
)

// CheckRequest is what the caller sends.
type CheckRequest struct {
	Key       string `json:"key"`
	Policy    string `json:"policy"`
	Limit     int64  `json:"limit"`
	WindowMs  int64  `json:"window_ms"`
	Algorithm string `json:"algorithm"`
}

// Algorithms holds both algorithm instances.
type Algorithms struct {
	SlidingWindow limiter.Algorithm
	GCRA          limiter.Algorithm
}

type PolicyStore interface {
	Get(name string) (policy.Policy, bool)
	List() []policy.Policy
}
