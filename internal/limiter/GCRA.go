package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// GCRA (Generic Cell Rate Algorithm)
//
// Core idea: every key has a "theoretical arrival time" (TAT) stored in Redis.
// TAT = the earliest moment the next request is allowed.
//
// On each request:
//   - If now >= TAT → allowed, update TAT forward by one emission interval
//   - If now < TAT  → denied, tell caller how long to wait
//
// emissionInterval = windowMs / limit
// e.g. 100 req/min → 60000ms / 100 = 600ms per slot
//
// One string key per user in Redis. No sorted sets. Memory efficient.

type GCRA struct {
	rdb *redis.Client
}

func NewGCRA(rdb *redis.Client) *GCRA {
	return &GCRA{rdb: rdb}
}

func (g *GCRA) Check(ctx context.Context, key string, limit, window int64) Result {
	now := time.Now().UnixMilli()

	// How many ms each request slot occupies
	emissionInterval := window / limit

	// Burst tolerance — how far behind TAT we still allow
	// Allows up to limit-1 requests instantly before smoothing kicks in
	burst := emissionInterval * (limit - 1)

	tatKey := "gcra:" + key

	// Get current TAT from Redis
	tat, err := g.rdb.Get(ctx, tatKey).Int64()
	if err == redis.Nil {
		// First ever request for this key
		tat = now
	} else if err != nil {
		// Redis error — fail open so GoBouncer outage doesn't kill your app
		return Result{Allowed: true, Remaining: 0, RetryAfter: 0}
	}

	// The earliest TAT we still accept (burst tolerance window)
	allowAt := tat - burst

	if now < allowAt {
		// Too early — deny and tell caller how long to wait
		retryAfter := allowAt - now
		return Result{Allowed: false, Remaining: 0, RetryAfter: retryAfter}
	}

	// Allowed — push TAT forward by one emission interval
	newTAT := tat
	if now > tat {
		newTAT = now
	}
	newTAT += emissionInterval

	// How many more requests fit before burst is exhausted (rounded up division)
	remaining := limit - (newTAT-now+emissionInterval-1)/emissionInterval
	if remaining < 0 {
		remaining = 0
	}

	// Persist new TAT to Redis with expiry
	g.rdb.Set(ctx, tatKey, newTAT, time.Duration(window)*time.Millisecond)

	return Result{Allowed: true, Remaining: remaining, RetryAfter: 0}
}
