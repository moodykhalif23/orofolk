package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the body; MaxBytesReader returns an error once the cap is
		// exceeded, which a real handler surfaces as 413 (mirroring how a JSON
		// decoder would fail on an oversized payload).
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestSecureHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	SecureHeaders(okHandler()).ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	// HSTS must NOT be set on a plain-HTTP request.
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS set on plain HTTP: %q", got)
	}
}

func TestMaxBytes(t *testing.T) {
	h := MaxBytes(16)(okHandler())

	// Under the limit: 200.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small")))
	if rec.Code != http.StatusOK {
		t.Errorf("under-limit status = %d, want 200", rec.Code)
	}

	// Over the limit: the handler's read fails, surfacing as 413.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 64))))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("over-limit status = %d, want 413", rec.Code)
	}
}

func TestRateLimit(t *testing.T) {
	h := RateLimit(3, time.Minute)(okHandler())
	call := func() int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.7:5555"
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 3; i++ {
		if code := call(); code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i+1, code)
		}
	}
	if code := call(); code != http.StatusTooManyRequests {
		t.Errorf("4th request = %d, want 429", code)
	}

	// A different client IP has its own budget.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "198.51.100.1:5555"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("distinct IP = %d, want 200", rec.Code)
	}
}
