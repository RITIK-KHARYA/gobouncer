package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Result struct {
	Allowed    bool  `json:"allowed"`
	Remaining  int64 `json:"remaining"`
	RetryAfter int64 `json:"retry_after,omitempty"`
}

type Limiter struct {
	rdb *redis.Client
	ctx context.Context
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb, ctx: context.Background()}
}

func (l *Limiter) Check(key string, window, limit int64) Result {
	now := time.Now().UnixMilli()
	windowStart := now - window

	// Step 1: prune old entries and count current ones
	pipe := l.rdb.Pipeline()
	pipe.ZRemRangeByScore(l.ctx, key, "0", fmt.Sprintf("%d", windowStart))
	countcmd := pipe.ZCard(l.ctx, key)
	_, err := pipe.Exec(l.ctx)
	if err != nil {
		return Result{Allowed: false, Remaining: 0, RetryAfter: 0}
	}

	count := countcmd.Val()

	// Step 2: reject if at or over limit
	if count >= limit {
		oldest, _ := l.rdb.ZRangeWithScores(l.ctx, key, 0, 0).Result()
		retryAfter := window
		if len(oldest) > 0 {
			retryAfter = int64(oldest[0].Score) + window - now
		}
		
		return Result{Allowed: false, Remaining: 0, RetryAfter: retryAfter}
	}

	// Step 3: under limit — record the request
	pipe2 := l.rdb.Pipeline()
	pipe2.ZAdd(l.ctx, key, redis.Z{Score: float64(now), Member: now})
	pipe2.Expire(l.ctx, key, time.Duration(window)*time.Millisecond)
	pipe2.Exec(l.ctx)

	return Result{Allowed: true, Remaining: limit-count-1, RetryAfter: 0}
}