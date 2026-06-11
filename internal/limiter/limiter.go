package limiter

import "context"

type Result struct {
	Allowed    bool  `json:"allowed"`
	Remaining  int64 `json:"remaining"`
	RetryAfter int64 `json:"retry_after,omitempty"`
}

type Algorithm interface {
	Check(ctx context.Context, key string, limit, window int64) Result
}
