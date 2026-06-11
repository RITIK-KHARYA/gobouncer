package limiter_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/ritik-kharya/gobouncer/internal/limiter"
)

func setupSlidingWindow(t *testing.T) (*limiter.SlidingWindow, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return limiter.NewSlidingWindow(rdb), mr
}

func TestSlidingWindow_FirstRequest_Allowed(t *testing.T) {
	sw, _ := setupSlidingWindow(t)
	ctx := context.Background()

	result := sw.Check(ctx, "user:1", 10, 60000) // 10 req per 60s

	if !result.Allowed {
		t.Fatal("expected first request to be allowed")
	}
	if result.Remaining != 9 {
		t.Fatalf("expected remaining=9, got %d", result.Remaining)
	}
	if result.RetryAfter != 0 {
		t.Fatalf("expected retry_after=0, got %d", result.RetryAfter)
	}
}

func TestSlidingWindow_ExactLimit_Denied(t *testing.T) {
	sw, _ := setupSlidingWindow(t)
	ctx := context.Background()

	limit := int64(5)
	window := int64(60000)

	// Use up all 5 slots
	for i := int64(0); i < limit; i++ {
		result := sw.Check(ctx, "user:2", limit, window)
		if !result.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
		expectedRemaining := limit - i - 1
		if result.Remaining != expectedRemaining {
			t.Fatalf("request %d: expected remaining=%d, got %d", i+1, expectedRemaining, result.Remaining)
		}
	}

	// 6th request should be denied
	result := sw.Check(ctx, "user:2", limit, window)
	if result.Allowed {
		t.Fatal("request beyond limit should be denied")
	}
	if result.Remaining != 0 {
		t.Fatalf("expected remaining=0 when denied, got %d", result.Remaining)
	}
	if result.RetryAfter <= 0 {
		t.Fatal("expected retry_after > 0 when denied")
	}
}

func TestSlidingWindow_DifferentKeys_Independent(t *testing.T) {
	sw, _ := setupSlidingWindow(t)
	ctx := context.Background()

	// Fill up key A
	for i := 0; i < 3; i++ {
		sw.Check(ctx, "user:A", 3, 60000)
	}

	// Key B should still be allowed
	result := sw.Check(ctx, "user:B", 3, 60000)
	if !result.Allowed {
		t.Fatal("different keys should be independent")
	}
	if result.Remaining != 2 {
		t.Fatalf("expected remaining=2 for fresh key, got %d", result.Remaining)
	}
}

func TestSlidingWindow_WindowExpiry_Resets(t *testing.T) {
	sw, mr := setupSlidingWindow(t)
	ctx := context.Background()

	// Use up all slots
	for i := 0; i < 3; i++ {
		sw.Check(ctx, "user:3", 3, 1000) // 1 second window
	}

	// Should be denied
	result := sw.Check(ctx, "user:3", 3, 1000)
	if result.Allowed {
		t.Fatal("should be denied at limit")
	}

	// Fast-forward time past the window
	mr.FastForward(2 * time.Second)

	// Should be allowed again
	result = sw.Check(ctx, "user:3", 3, 1000)
	if !result.Allowed {
		t.Fatal("should be allowed after window expiry")
	}
}

func TestSlidingWindow_RemainingCountsDown(t *testing.T) {
	sw, _ := setupSlidingWindow(t)
	ctx := context.Background()

	limit := int64(5)
	for i := int64(0); i < limit; i++ {
		result := sw.Check(ctx, "user:countdown", limit, 60000)
		expected := limit - i - 1
		if result.Remaining != expected {
			t.Fatalf("request %d: expected remaining=%d, got %d", i+1, expected, result.Remaining)
		}
	}
}
