package authmod_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/server"
	"b2bcommerce/internal/store"
	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
)

const testSecret = "test-secret-please-change"

func newServer(t *testing.T) (http.Handler, *pgxpool.Pool, *auth.Issuer) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), pool, issuer
}

func post(t *testing.T, h http.Handler, path, tok string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// seedVendor creates a vendor + one vendor-portal login.
func seedVendor(t *testing.T, pool *pgxpool.Pool, name, slug, email, password string) gen.Vendor {
	t.Helper()
	q := gen.New(pool)
	ctx := context.Background()
	v, err := q.CreateVendor(ctx, gen.CreateVendorParams{
		OrganizationID: 1, Name: name, Slug: slug, CommissionRate: "10",
	})
	if err != nil {
		t.Fatalf("create vendor: %v", err)
	}
	hash, _ := auth.HashPassword(password)
	if _, err := q.CreateVendorUser(ctx, gen.CreateVendorUserParams{
		VendorID: v.ID, Email: email, PasswordHash: hash, FullName: name + " Owner",
	}); err != nil {
		t.Fatalf("create vendor user: %v", err)
	}
	return v
}

// TestVendorLogin proves the third (vendor) token audience works end to end:
// a vendor-user authenticates, the minted token carries the vendor + org, wrong
// credentials are rejected, and a vendor token cannot reach an admin route.
func TestVendorLogin(t *testing.T) {
	h, pool, issuer := newServer(t)
	const email, password = "owner@acme-supply.test", "vendor-pass-123"
	v := seedVendor(t, pool, "Acme Supply", "acme-supply", email, password)

	// Happy path: valid credentials mint a vendor token.
	rr := post(t, h, "/vendor/auth/login", "", map[string]any{"email": email, "password": password, "org_id": 1})
	if rr.Code != http.StatusOK {
		t.Fatalf("vendor login: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.Token == "" {
		t.Fatalf("decode token: %v (%s)", err, rr.Body.String())
	}

	claims, err := issuer.Parse(resp.Token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Audience != "vendor" {
		t.Errorf("audience: want vendor, got %q", claims.Audience)
	}
	if claims.VendorID != v.ID {
		t.Errorf("vendor_id: want %d, got %d", v.ID, claims.VendorID)
	}
	if claims.OrgID != 1 {
		t.Errorf("org_id: want 1, got %d", claims.OrgID)
	}

	// Wrong password is rejected.
	if rr := post(t, h, "/vendor/auth/login", "", map[string]any{"email": email, "password": "nope", "org_id": 1}); rr.Code != http.StatusUnauthorized {
		t.Errorf("bad password: want 401, got %d", rr.Code)
	}

	// Audience gating: a vendor token must not be accepted on an admin route.
	req := httptest.NewRequest(http.MethodGet, "/admin/orders", nil)
	req.Header.Set("Authorization", "Bearer "+resp.Token)
	ar := httptest.NewRecorder()
	h.ServeHTTP(ar, req)
	if ar.Code != http.StatusForbidden {
		t.Errorf("vendor token on admin route: want 403, got %d", ar.Code)
	}
}
