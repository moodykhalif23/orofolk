package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"b2bcommerce/internal/server/response"
)

// rateLimiter is a small dependency-free fixed-window limiter keyed by a string
// (typically client IP). It is intended for low-volume sensitive endpoints such
// as login, to blunt credential-stuffing and brute-force attempts. It is
// per-process; behind multiple instances each enforces its own window.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string]*window
	limit  int
	window time.Duration
}

type window struct {
	count int
	reset time.Time
}

// RateLimit returns middleware allowing at most `limit` requests per `per`
// duration from a single client IP. Excess requests get 429 with a Retry-After
// header. A background sweep evicts stale windows so the map cannot grow without
// bound.
func RateLimit(limit int, per time.Duration) func(http.Handler) http.Handler {
	rl := &rateLimiter{hits: make(map[string]*window), limit: limit, window: per}
	go rl.sweep()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if retryAfter, ok := rl.allow(clientKey(r)); !ok {
				w.Header().Set("Retry-After", retryAfter)
				response.Fail(w, http.StatusTooManyRequests, "rate_limited", "too many requests, slow down")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *rateLimiter) allow(key string) (string, bool) {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	wnd, ok := rl.hits[key]
	if !ok || now.After(wnd.reset) {
		rl.hits[key] = &window{count: 1, reset: now.Add(rl.window)}
		return "", true
	}
	if wnd.count >= rl.limit {
		secs := int(time.Until(wnd.reset).Seconds()) + 1
		return itoa(secs), false
	}
	wnd.count++
	return "", true
}

func (rl *rateLimiter) sweep() {
	t := time.NewTicker(rl.window)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		rl.mu.Lock()
		for k, wnd := range rl.hits {
			if now.After(wnd.reset) {
				delete(rl.hits, k)
			}
		}
		rl.mu.Unlock()
	}
}

// clientKey derives the limiter key from the request. RealIP middleware (run
// earlier) normalises X-Forwarded-For into RemoteAddr; strip the port.
func clientKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func itoa(n int) string {
	if n <= 0 {
		return "1"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
