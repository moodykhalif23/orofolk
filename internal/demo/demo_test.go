package demo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/demo"
	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
)

// TestProvisionSeededDemo proves an instant demo is provisioned, activated,
// seeded, and returns a working admin token for the new tenant.
func TestProvisionSeededDemo(t *testing.T) {
	pool := testsupport.NewDB(t)
	issuer := auth.NewIssuer("test-secret-please-change", time.Hour)
	ctx := context.Background()

	res, err := demo.Provision(ctx, pool, issuer, demo.Input{Email: "prospect@acme.test", Company: "Acme Co", Name: "Pat"})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if res.Token == "" {
		t.Fatal("expected an auto-login token")
	}
	if !strings.HasPrefix(res.Domain, "demo-") {
		t.Errorf("domain = %q, want a demo- prefix", res.Domain)
	}
	if !res.ExpiresAt.After(time.Now()) {
		t.Errorf("demo expiry should be in the future, got %v", res.ExpiresAt)
	}

	// The token authenticates as an admin of the freshly-created tenant.
	claims, err := issuer.Parse(res.Token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Audience != "admin" {
		t.Errorf("audience = %q, want admin", claims.Audience)
	}
	if claims.OrgID <= 1 {
		t.Errorf("org id = %d, want a brand-new tenant (>1)", claims.OrgID)
	}
	if len(claims.Permissions) == 0 {
		t.Error("expected the demo admin to carry seeded permissions")
	}

	// The org is live (no email verification) and flagged as a demo.
	q := gen.New(pool)
	org, err := q.GetOrganization(ctx, claims.OrgID)
	if err != nil {
		t.Fatalf("get org: %v", err)
	}
	if !org.IsDemo {
		t.Error("org should be flagged is_demo")
	}
	if org.Status != "active" {
		t.Errorf("status = %q, want active", org.Status)
	}

	// Seeded catalog so the dashboards/insights are alive on first login.
	n, err := q.CountProductsAdmin(ctx, claims.OrgID)
	if err != nil {
		t.Fatalf("count products: %v", err)
	}
	if n == 0 {
		t.Error("expected the demo org to be seeded with products")
	}
}
