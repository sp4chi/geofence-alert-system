package httpapi

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a token bucket per client. Each client earns `rate` tokens per second up to
// `burst`; every request spends one. Steady traffic at or below the rate always passes, short
// bursts are absorbed, and a flood gets 429 until tokens refill. Off unless RATE_LIMIT_RPS > 0.
type rateLimiter struct {
	rate  float64
	burst float64
	now   func() time.Time // a field so tests can control the clock

	mu        sync.Mutex // guards everything below; handlers run concurrently
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter allows bursts of twice the per-second rate (at least 1), e.g. 5 rps → bursts of 10.
func newRateLimiter(rps float64) *rateLimiter {
	return &rateLimiter{
		rate:    rps,
		burst:   math.Max(1, math.Ceil(2*rps)),
		now:     time.Now,
		buckets: make(map[string]*bucket),
	}
}

// allow spends a token for key. If none is left it reports how long until the next one.
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	// Refill lazily for the time since this client's last request, capped at the burst size.
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
}

// sweep forgets clients idle long enough for their bucket to be full again: a new bucket would
// behave identically, so dropping them keeps memory bounded without changing behaviour.
func (l *rateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	refillTime := time.Duration(l.burst / l.rate * float64(time.Second))
	for key, b := range l.buckets {
		if now.Sub(b.last) > refillTime {
			delete(l.buckets, key)
		}
	}
}

// limit wraps one handler (POST /vehicles/location) with the limiter.
func (l *rateLimiter) limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if allowed, wait := l.allow(clientIP(r)); !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			respondError(w, http.StatusTooManyRequests, start, "rate limit exceeded: too many location updates, retry shortly")
			return
		}
		next(w, r)
	}
}

// clientIP identifies the caller. Behind Render/Cloudflare the real client is the first
// X-Forwarded-For entry; without a proxy that header is client-controlled, so this is a fairness
// limit for well-behaved devices, not a security boundary.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
