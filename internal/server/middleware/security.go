package middleware

import (
	"net/http"
	"strings"
)

// SecureHeaders sets conservative response headers on every request. HSTS is
// only emitted when the request arrived over TLS (or was forwarded as https),
// so it never breaks plain-HTTP local development.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// MaxBytes caps the size of request bodies the server will read, guarding
// against accidental or malicious oversized payloads. A handler that reads past
// the limit gets an error from the body reader (surfaced as 400 by decoders).
// Multipart uploads (file uploads, e.g. DAM media) are exempt — those routes
// apply their own, larger limit — so the JSON cap can stay tight.
func MaxBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}
