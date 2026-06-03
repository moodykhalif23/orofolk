package tenancy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func newServer(t *testing.T) (http.Handler, *auth.Issuer, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), issuer, pool
}

// req optionally sets the Host header (for storefront website resolution).
func req(t *testing.T, h http.Handler, method, path, tok, host string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	if host != "" {
		r.Host = host
	}
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	return rr
}

func tenantToken(t *testing.T, issuer *auth.Issuer, perms ...string) string {
	t.Helper()
	tok, _ := issuer.Issue("1", 1, "admin", perms)
	return tok
}

func TestWebsiteCRUD(t *testing.T) {
	h, issuer, _ := newServer(t)
	tok := tenantToken(t, issuer, "tenant.view", "tenant.manage")

	// Seed has one website for org 1.
	rr := req(t, h, http.MethodGet, "/admin/websites", tok, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d (%s)", rr.Code, rr.Body.String())
	}
	var list struct {
		Items []struct{ ID int64 } `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list.Items) < 1 {
		t.Fatalf("seed website missing")
	}

	// Create a second website.
	cr := req(t, h, http.MethodPost, "/admin/websites", tok, "", map[string]any{
		"name": "EU Store", "domain": "eu.teggo.test", "default_currency": "EUR", "default_locale": "fr",
	})
	if cr.Code != http.StatusCreated {
		t.Fatalf("create: %d (%s)", cr.Code, cr.Body.String())
	}
	var ws struct {
		ID              int64  `json:"id"`
		Domain          string `json:"domain"`
		DefaultCurrency string `json:"default_currency"`
	}
	_ = json.Unmarshal(cr.Body.Bytes(), &ws)
	if ws.DefaultCurrency != "EUR" {
		t.Errorf("currency: want EUR, got %s", ws.DefaultCurrency)
	}

	// Update it.
	up := req(t, h, http.MethodPut, "/admin/websites/"+strconv.FormatInt(ws.ID, 10), tok, "", map[string]any{
		"name": "EU Store", "domain": "eu.teggo.test", "default_currency": "EUR", "default_locale": "de",
	})
	if up.Code != http.StatusOK {
		t.Fatalf("update: %d (%s)", up.Code, up.Body.String())
	}

	// Duplicate domain → conflict.
	if dup := req(t, h, http.MethodPost, "/admin/websites", tok, "", map[string]any{
		"name": "Dup", "domain": "eu.teggo.test", "default_currency": "USD",
	}); dup.Code != http.StatusConflict {
		t.Errorf("duplicate domain: want 409, got %d", dup.Code)
	}
}

func TestWebsiteHostResolution(t *testing.T) {
	h, _, pool := newServer(t)
	q := gen.New(pool)
	ctx := context.Background()

	// A second org with its own website + product.
	var org2 int64
	if err := pool.QueryRow(ctx, `INSERT INTO organizations (name) VALUES ('Org Two') RETURNING id`).Scan(&org2); err != nil {
		t.Fatalf("org2: %v", err)
	}
	if _, err := q.CreateWebsite(ctx, gen.CreateWebsiteParams{
		OrganizationID: org2, Name: "Org2 Store", Domain: "org2.test", DefaultCurrency: "USD", DefaultLocale: "en",
	}); err != nil {
		t.Fatalf("website: %v", err)
	}
	if _, err := q.CreateProduct(ctx, gen.CreateProductParams{
		OrganizationID: org2, Sku: "O2-1", Type: "simple", Name: "Org Two Widget", Slug: "o2-1", Status: "active", Attributes: []byte("{}"), Unit: "each",
	}); err != nil {
		t.Fatalf("product: %v", err)
	}

	// Request with org2's host → org2's catalog (the one product, not org-1's seed).
	rr := req(t, h, http.MethodGet, "/storefront/products", "", "org2.test", nil)
	var resp struct {
		Items []struct{ Sku string } `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Sku != "O2-1" {
		t.Fatalf("host routing: want only O2-1 for org2.test, got %+v", resp.Items)
	}

	// Unknown host → fall back to the demo org (org 1, seeded with 2 active products).
	fb := req(t, h, http.MethodGet, "/storefront/products", "", "unknown.test", nil)
	var fbResp struct {
		Items []struct{ Sku string } `json:"items"`
	}
	_ = json.Unmarshal(fb.Body.Bytes(), &fbResp)
	if len(fbResp.Items) != 2 {
		t.Errorf("unknown host fallback to org 1: want 2 seeded products, got %d", len(fbResp.Items))
	}
}

func TestTenancyAuth(t *testing.T) {
	h, issuer, _ := newServer(t)
	// Missing tenant.view.
	if rr := req(t, h, http.MethodGet, "/admin/websites", tenantToken(t, issuer, "order.view"), "", nil); rr.Code != http.StatusForbidden {
		t.Errorf("no permission: want 403, got %d", rr.Code)
	}
	// Storefront token (wrong audience).
	cust, _ := issuer.IssueStorefront(0, 1, 1)
	if rr := req(t, h, http.MethodGet, "/admin/websites", cust, "", nil); rr.Code != http.StatusForbidden {
		t.Errorf("storefront token: want 403, got %d", rr.Code)
	}
}
