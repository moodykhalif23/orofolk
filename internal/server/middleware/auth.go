package middleware

import (
	"context"
	"net/http"
	"strings"

	"b2bcommerce/internal/apikey"
	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/tenantctx"
)

type ctxKey int

const claimsKey ctxKey = iota

// KeyVerifier resolves a raw API-key bearer token ("tgk_…") to claims. It is
// implemented by *apikey.Service. A nil verifier disables API-key auth, leaving
// JWT-only behaviour.
type KeyVerifier interface {
	VerifyKey(ctx context.Context, raw string) (*auth.Claims, error)
}

// Authenticator parses the Bearer token (JWT only) and stores claims in the
// request context. It rejects requests without a valid token.
func Authenticator(issuer *auth.Issuer) func(http.Handler) http.Handler {
	return AuthenticatorWithKeys(issuer, nil)
}

// AuthenticatorWithKeys is Authenticator that also accepts programmatic API
// keys: a bearer value with the apikey.Prefix is verified against the key store,
// anything else is parsed as a JWT. Either path yields claims that are used
// identically downstream (audience, permission, org-arming).
func AuthenticatorWithKeys(issuer *auth.Issuer, keys KeyVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				response.Fail(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			token := strings.TrimPrefix(h, "Bearer ")
			claims, err := resolveClaims(r.Context(), issuer, keys, token)
			if err != nil {
				response.Fail(w, http.StatusUnauthorized, "unauthorized", "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			ctx = tenantctx.WithOrg(ctx, claims.OrgID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolveClaims picks API-key vs JWT verification by the token's shape.
func resolveClaims(ctx context.Context, issuer *auth.Issuer, keys KeyVerifier, token string) (*auth.Claims, error) {
	if keys != nil && strings.HasPrefix(token, apikey.Prefix) {
		return keys.VerifyKey(ctx, token)
	}
	return issuer.Parse(token)
}

// OptionalAuthenticator parses a Bearer token when present and stores the
// claims in context, but does NOT reject anonymous requests. Used on public
// storefront reads (e.g. catalog) that personalize for a signed-in buyer
// (per-customer catalog visibility) yet must still serve anonymous visitors.
func OptionalAuthenticator(issuer *auth.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if strings.HasPrefix(h, "Bearer ") {
				if claims, err := issuer.Parse(strings.TrimPrefix(h, "Bearer ")); err == nil {
					ctx := context.WithValue(r.Context(), claimsKey, claims)
					r = r.WithContext(tenantctx.WithOrg(ctx, claims.OrgID))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsFrom returns the claims stored by Authenticator, if present.
func ClaimsFrom(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*auth.Claims)
	return c, ok
}
