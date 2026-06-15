// Package apikey mints and verifies programmatic API keys (Platform roadmap,
// Phase 0). A key authenticates a back-office/integration caller in place of a
// user JWT: the bearer value "tgk_<random>" hashes to a stored SHA-256 digest
// (the raw key is never persisted), and the key's scopes — permission strings
// from the same catalog as roles — become the request's claims, so the existing
// RequirePermission middleware gates key traffic unchanged.
package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/store/gen"
)

// Prefix marks a Teggo API key bearer token, distinguishing it from a JWT.
const Prefix = "tgk_"

// prefixLen is how many leading characters of a key we store/display so a user
// can recognise a key without it revealing the secret.
const prefixLen = 12

// ErrInvalid is returned when a presented key does not resolve to a live key.
var ErrInvalid = errors.New("invalid api key")

// Service verifies and generates API keys against the database.
type Service struct {
	q *gen.Queries
}

// NewService builds an API-key service over the connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: gen.New(pool)}
}

// Generate returns a fresh raw key, its display prefix, and its storage hash.
// The raw key is returned to the caller exactly once (at creation/rotation).
func Generate() (raw, prefix, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", err
	}
	raw = Prefix + hex.EncodeToString(buf)
	prefix = raw
	if len(raw) > prefixLen {
		prefix = raw[:prefixLen]
	}
	return raw, prefix, Hash(raw), nil
}

// Hash is the SHA-256 hex digest stored for a raw key.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// VerifyKey resolves a raw bearer key to synthesized admin claims carrying the
// key's org and scopes, and records last-use (debounced). It returns ErrInvalid
// for any token that is not a live key. Satisfies middleware.KeyVerifier.
//
// The lookup runs before any org is armed on the request, so it relies on the
// globally-unique key_hash (RLS fails open with no app.org_id set) — the same
// pattern the inbound ERP webhook uses to resolve a connection by id.
func (s *Service) VerifyKey(ctx context.Context, raw string) (*auth.Claims, error) {
	if !strings.HasPrefix(raw, Prefix) {
		return nil, ErrInvalid
	}
	row, err := s.q.GetAPIKeyByHash(ctx, Hash(raw))
	if err != nil {
		return nil, ErrInvalid
	}
	_ = s.q.TouchAPIKey(ctx, row.ID) // best-effort last-used stamp
	return &auth.Claims{
		OrgID:       row.OrganizationID,
		Audience:    "admin",
		Permissions: row.Scopes,
	}, nil
}
