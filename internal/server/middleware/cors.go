package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

func CORS(allowed []string) func(http.Handler) http.Handler {
	allowAny := false
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAny = true
		}
		set[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" || (len(set) == 0) {
				next.ServeHTTP(w, r)
				return
			}

			_, ok := set[origin]
			if !ok && !allowAny {
				// Not an allowed origin: don't emit CORS headers. The browser
				// blocks the response, which is the correct outcome.
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()
			// Reflect the specific origin (never bare "*") so the response is
			// valid even when the client sends credentials, and add Vary so
			// shared caches key on Origin.
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				// Preflight: advertise the methods and headers the client asked
				// about, cache the decision, and stop here.
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
					h.Set("Access-Control-Allow-Headers", reqHeaders)
				} else {
					h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				}
				h.Set("Access-Control-Max-Age", strconv.Itoa(600))
				h.Add("Vary", "Access-Control-Request-Method")
				h.Add("Vary", "Access-Control-Request-Headers")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
