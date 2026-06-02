package middleware

import (
	"net/http"

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
