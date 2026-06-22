package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ritik-kharya/gobouncer/internal/limiter"
	"github.com/ritik-kharya/gobouncer/internal/policy"
)

type stubAlgorithm struct {
	result limiter.Result
	called bool
	key    string
	limit  int64
	window int64
	calls  []stubCall
}

type stubCall struct {
	key    string
	limit  int64
	window int64
}

func (s *stubAlgorithm) Check(ctx context.Context, key string, limit, window int64) limiter.Result {
	s.called = true
	s.key = key
	s.limit = limit
	s.window = window
	s.calls = append(s.calls, stubCall{key: key, limit: limit, window: window})
	return s.result
}

func testPolicies(t *testing.T) *policy.MemoryStore {
	t.Helper()
	store, err := policy.NewMemoryStore([]policy.Policy{
		{Name: "login", Limit: 5, WindowMs: 300_000, Algorithm: policy.AlgorithmGCRA},
		{Name: "ip-basic", Limit: 100, WindowMs: 60_000, Algorithm: policy.AlgorithmGCRA},
		{Name: "user-free", Limit: 1000, WindowMs: 86_400_000, Algorithm: policy.AlgorithmGCRA},
	})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestCheckHandler_PolicyRequestUsesPolicySettings(t *testing.T) {
	sliding := &stubAlgorithm{result: limiter.Result{Allowed: true, Remaining: 99}}
	gcra := &stubAlgorithm{result: limiter.Result{Allowed: true, Remaining: 4}}
	handler := NewCheckHandler(Algorithms{SlidingWindow: sliding, GCRA: gcra}, testPolicies(t))

	body := bytes.NewBufferString(`{"key":"user:123","policy":"login"}`)
	req := httptest.NewRequest(http.MethodPost, "/check", body)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if sliding.called {
		t.Fatal("sliding window should not be called for gcra policy")
	}
	if !gcra.called {
		t.Fatal("gcra should be called")
	}
	if gcra.key != "user:123" || gcra.limit != 5 || gcra.window != 300_000 {
		t.Fatalf("unexpected limiter call: key=%q limit=%d window=%d", gcra.key, gcra.limit, gcra.window)
	}
	if got := rr.Header().Get("X-RateLimit-Policy"); got != "login" {
		t.Fatalf("expected policy header login, got %q", got)
	}
	if got := rr.Header().Get("X-RateLimit-Limit"); got != "5" {
		t.Fatalf("expected limit header 5, got %q", got)
	}
}

func TestCheckHandler_RawLimitRequestStillWorks(t *testing.T) {
	sliding := &stubAlgorithm{result: limiter.Result{Allowed: true, Remaining: 9}}
	handler := NewCheckHandler(Algorithms{SlidingWindow: sliding}, testPolicies(t))

	body := bytes.NewBufferString(`{"key":"ip:127.0.0.1","limit":10,"window_ms":60000}`)
	req := httptest.NewRequest(http.MethodPost, "/check", body)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !sliding.called {
		t.Fatal("sliding window should be called")
	}
	if sliding.limit != 10 || sliding.window != 60_000 {
		t.Fatalf("unexpected limiter call: limit=%d window=%d", sliding.limit, sliding.window)
	}
}

func TestCheckHandler_UnknownPolicyReturnsNotFound(t *testing.T) {
	handler := NewCheckHandler(Algorithms{SlidingWindow: &stubAlgorithm{}}, testPolicies(t))

	body := bytes.NewBufferString(`{"key":"user:123","policy":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/check", body)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestCheckHandler_MultiCheckRequiresAllDimensions(t *testing.T) {
	gcra := &stubAlgorithm{result: limiter.Result{Allowed: true, Remaining: 4}}
	handler := NewCheckHandler(Algorithms{GCRA: gcra}, testPolicies(t))

	body := bytes.NewBufferString(`{"checks":[{"name":"ip","key":"ip:127.0.0.1","policy":"ip-basic"},{"name":"user","key":"user:123","policy":"user-free"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/check", body)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(gcra.calls) != 2 {
		t.Fatalf("expected 2 limiter calls, got %d", len(gcra.calls))
	}
	if gcra.calls[0].key != "ip:127.0.0.1" || gcra.calls[0].limit != 100 {
		t.Fatalf("unexpected first check: %+v", gcra.calls[0])
	}
	if gcra.calls[1].key != "user:123" || gcra.calls[1].limit != 1000 {
		t.Fatalf("unexpected second check: %+v", gcra.calls[1])
	}

	var response MultiCheckResult
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Allowed {
		t.Fatal("expected multi-check to be allowed")
	}
	if len(response.Checks) != 2 {
		t.Fatalf("expected 2 check results, got %d", len(response.Checks))
	}
}

func TestCheckHandler_MultiCheckDeniesWhenAnyDimensionDenies(t *testing.T) {
	gcra := &stubAlgorithm{result: limiter.Result{Allowed: false, Remaining: 0, RetryAfter: 1500}}
	handler := NewCheckHandler(Algorithms{GCRA: gcra}, testPolicies(t))

	body := bytes.NewBufferString(`{"checks":[{"name":"login","key":"route:/login:user:123","policy":"login"},{"name":"user","key":"user:123","policy":"user-free"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/check", body)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(gcra.calls) != 1 {
		t.Fatalf("expected short-circuit after first denied check, got %d calls", len(gcra.calls))
	}
	if got := rr.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected Retry-After=1, got %q", got)
	}

	var response MultiCheckResult
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Allowed {
		t.Fatal("expected multi-check to be denied")
	}
	if len(response.Checks) != 1 || response.Checks[0].Name != "login" {
		t.Fatalf("unexpected check results: %+v", response.Checks)
	}
}

func TestPoliciesHandler_ReturnsPolicies(t *testing.T) {
	handler := NewPoliciesHandler(testPolicies(t))
	req := httptest.NewRequest(http.MethodGet, "/policies", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var response struct {
		Policies []policy.Policy `json:"policies"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	foundLogin := false
	for _, p := range response.Policies {
		if p.Name == "login" {
			foundLogin = true
			break
		}
	}
	if !foundLogin {
		t.Fatalf("unexpected policies response: %+v", response.Policies)
	}
}
