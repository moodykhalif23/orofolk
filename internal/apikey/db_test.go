package apikey

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
)

// org 1 is the demo organization seeded by migration 0003; its admin role is the
// permission template every tenant inherits from.
const demoOrg = 1

// TestVerifyKeyRoundTrip exercises the real authentication path against Postgres:
// a key minted into api_keys resolves to claims carrying its org + scopes, and
// stops resolving once revoked or expired.
func TestVerifyKeyRoundTrip(t *testing.T) {
	pool := testsupport.NewDB(t)
	q := gen.New(pool)
	svc := NewService(pool)
	ctx := context.Background()

	raw, prefix, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	scopes := []string{"product.view", "order.view"}
	row, err := q.CreateAPIKey(ctx, gen.CreateAPIKeyParams{
		OrganizationID: demoOrg, Name: "test key", Prefix: prefix, KeyHash: hash, Scopes: scopes,
	})
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	claims, err := svc.VerifyKey(ctx, raw)
	if err != nil {
		t.Fatalf("VerifyKey live key: %v", err)
	}
	if claims.OrgID != demoOrg {
		t.Errorf("OrgID = %d, want %d", claims.OrgID, demoOrg)
	}
	if claims.Audience != "admin" {
		t.Errorf("Audience = %q, want admin", claims.Audience)
	}
	if len(claims.Permissions) != 2 || claims.Permissions[0] != "product.view" {
		t.Errorf("Permissions = %v, want %v", claims.Permissions, scopes)
	}

	// Wrong shape and unknown hash are rejected without a DB hit / match.
	if _, err := svc.VerifyKey(ctx, "not-a-teggo-key"); err == nil {
		t.Error("expected ErrInvalid for a non-prefixed token")
	}
	if _, err := svc.VerifyKey(ctx, Prefix+"deadbeef"); err == nil {
		t.Error("expected ErrInvalid for an unknown key")
	}

	// Revoked → no longer authenticates.
	if err := q.RevokeAPIKey(ctx, gen.RevokeAPIKeyParams{OrganizationID: demoOrg, ID: row.ID}); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	if _, err := svc.VerifyKey(ctx, raw); err == nil {
		t.Error("expected ErrInvalid after revoke")
	}
}

// TestVerifyKeyExpired confirms an elapsed expiry stops authentication.
func TestVerifyKeyExpired(t *testing.T) {
	pool := testsupport.NewDB(t)
	q := gen.New(pool)
	svc := NewService(pool)
	ctx := context.Background()

	raw, prefix, hash, _ := Generate()
	_, err := q.CreateAPIKey(ctx, gen.CreateAPIKeyParams{
		OrganizationID: demoOrg, Name: "expired", Prefix: prefix, KeyHash: hash, Scopes: []string{"product.view"},
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if _, err := svc.VerifyKey(ctx, raw); err == nil {
		t.Error("expected ErrInvalid for an expired key")
	}
}

// TestBackfillGrantedPerms confirms migration 0066 reached the demo admin role
// (a proxy for every existing tenant's admin, which the same statement covers).
func TestBackfillGrantedPerms(t *testing.T) {
	pool := testsupport.NewDB(t)
	ctx := context.Background()

	var n int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id
		WHERE r.organization_id = $1 AND r.name = 'admin'
		  AND rp.permission = ANY($2)`,
		demoOrg, []string{"apikey.view", "apikey.manage", "webhook.view", "webhook.manage"},
	).Scan(&n)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 4 {
		t.Errorf("admin role holds %d of the 4 apikey/webhook perms, want 4", n)
	}
}
