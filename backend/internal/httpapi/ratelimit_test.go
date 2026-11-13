package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeClock lets tests move time forward instead of sleeping.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func limiterWithClock(rps float64) (*rateLimiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)}
	l := newRateLimiter(rps)
	l.now = clock.now
	return l, clock
}

func TestRateLimiterBurstThenRefill(t *testing.T) {
	l, clock := limiterWithClock(2) // 2 per second, bursts of 4

	for i := range 4 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("request %d within the burst was rejected", i+1)
		}
	}
	ok, wait := l.allow("a")
	if ok {
		t.Fatal("5th immediate request should be rejected")
	}
	if wait != 500*time.Millisecond {
		t.Errorf("wait = %v, want 500ms (one token at 2/s)", wait)
	}

	clock.advance(500 * time.Millisecond)
	if ok, _ := l.allow("a"); !ok {
		t.Error("a token should have refilled after 500ms")
	}
	if ok, _ := l.allow("a"); ok {
		t.Error("only one token should have refilled")
	}
}

func TestRateLimiterSteadyTrafficAtTheRateAlwaysPasses(t *testing.T) {
	l, clock := limiterWithClock(1)
	for i := range 20 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("request %d at exactly 1/s was rejected", i+1)
		}
		clock.advance(time.Second)
	}
}

func TestRateLimiterClientsAreIndependent(t *testing.T) {
	l, _ := limiterWithClock(0.5) // burst of 1
	if ok, _ := l.allow("a"); !ok {
		t.Fatal("first request from a rejected")
	}
	if ok, _ := l.allow("a"); ok {
		t.Fatal("second request from a should be limited")
	}
	if ok, _ := l.allow("b"); !ok {
		t.Error("client b must not be affected by client a")
	}
}

func TestRateLimiterSweepsIdleClients(t *testing.T) {
	l, clock := limiterWithClock(1) // burst 2 → full again after 2s
	l.allow("a")
	l.allow("b")
	clock.advance(2 * time.Minute)
	l.allow("c") // triggers the sweep
	if _, kept := l.buckets["a"]; kept || len(l.buckets) != 1 {
		t.Errorf("idle buckets not swept: %v", l.buckets)
	}
}

func TestRateLimiterIsSafeForConcurrentUse(t *testing.T) {
	l := newRateLimiter(1000)
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			for range 20 {
				l.allow("shared")
			}
		})
	}
	wg.Wait() // the race detector (make test) fails this if allow isn't properly locked
}

func TestRateLimitMiddlewareReturns429WithTimeNS(t *testing.T) {
	l, _ := limiterWithClock(0.5) // burst of 1
	handler := l.limit(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	send := func(forwardedFor string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/vehicles/location", nil)
		req.Header.Set("X-Forwarded-For", forwardedFor)
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}

	if rec := send("203.0.113.7"); rec.Code != http.StatusOK {
		t.Fatalf("first request: %d", rec.Code)
	}
	rec := send("203.0.113.7, 10.0.0.1") // same client behind a proxy chain
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "2" {
		t.Errorf("Retry-After = %q, want 2", rec.Header().Get("Retry-After"))
	}
	decode(t, rec) // JSON with numeric-string time_ns

	if rec := send("198.51.100.1"); rec.Code != http.StatusOK {
		t.Errorf("another client was limited: %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:54321"
	if got := clientIP(req); got != "192.0.2.10" {
		t.Errorf("without proxy: %q", got)
	}
	req.Header.Set("X-Forwarded-For", " 203.0.113.7 , 10.0.0.1")
	if got := clientIP(req); got != "203.0.113.7" {
		t.Errorf("behind proxy: %q", got)
	}
}

func TestRouterOnlyLimitsWhenConfigured(t *testing.T) {
	// With RATE_LIMIT_RPS=0 the route is not wrapped: many requests never get 429.
	// (They fail validation with 400 first, which needs no database.)
	router := NewRouter(Options{Health: fakePinger{}})
	for range 20 {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vehicles/location", nil))
		if rec.Code == http.StatusTooManyRequests {
			t.Fatal("rate limited although RATE_LIMIT_RPS is 0")
		}
	}

	limited := NewRouter(Options{Health: fakePinger{}, RateLimitRPS: 0.5})
	codes := []int{}
	for range 2 {
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/vehicles/location", nil))
		codes = append(codes, rec.Code)
	}
	if codes[0] != http.StatusBadRequest || codes[1] != http.StatusTooManyRequests {
		t.Errorf("codes = %v, want [400 429]", codes)
	}
}
