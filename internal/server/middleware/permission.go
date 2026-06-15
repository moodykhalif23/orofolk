package middleware

import (
	"net/http"
	"strings"

	"b2bcommerce/internal/server/response"
)

// RequireAudience ensures the token was minted for the expected context
// ("admin" or "storefront"), so an admin token can't be replayed against
// storefront routes (or vice versa). Must run after Authenticator.
func RequireAudience(aud string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFrom(r.Context())
			if !ok {
				response.Fail(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
				return
			}
			if claims.Audience != aud {
				response.Fail(w, http.StatusForbidden, "forbidden", "wrong token audience")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission ensures the authenticated principal holds the given
// permission. Must run after Authenticator.
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFrom(r.Context())
			if !ok {
				response.Fail(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
				return
			}
			for _, p := range claims.Permissions {
				if p == perm {
					next.ServeHTTP(w, r)
					return
				}
			}
			response.Fail(w, http.StatusForbidden, "forbidden", "missing permission: "+perm)
		})
	}
}

// RequireAnyPermission ensures the authenticated principal holds at least one of
// the given permissions. Used where two distinct roles legitimately reach the
// same endpoint — e.g. import target/template discovery, which both an
// interactive admin (import.view) and a scoped supplier key (import.ingest)
// need. Must run after Authenticator.
func RequireAnyPermission(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFrom(r.Context())
			if !ok {
				response.Fail(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
				return
			}
			for _, held := range claims.Permissions {
				for _, want := range perms {
					if held == want {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			response.Fail(w, http.StatusForbidden, "forbidden", "missing permission: "+strings.Join(perms, " or "))
		})
	}
}
