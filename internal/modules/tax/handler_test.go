package tax_test

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

const testSecret = "tax-test-secret"

func newServer(t *testing.T) (http.Handler, *auth.Issuer, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), issuer, pool
}

func tok(t *testing.T, issuer *auth.Issuer) string {
	t.Helper()
	s, _ := issuer.Issue("1", 1, "admin", []string{"tax.view", "tax.manage", "product.view", "product.manage"})
	return s
}

func do(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestTaxRatesAndCalculate(t *testing.T) {
	h, issuer, pool := newServer(t)
	at := tok(t, issuer)

	// Seed a product (default tax_class 'standard').
	p, err := gen.New(pool).CreateProduct(context.Background(), gen.CreateProductParams{
		OrganizationID: 1, Sku: "TAXED", Type: "simple", Name: "Taxed", Slug: "taxed",
		Status: "active", Attributes: []byte("{}"), Unit: "each",
	})
	if err != nil {
		t.Fatalf("product: %v", err)
	}

	// Configure 16% VAT for KE / standard.
	if rr := do(t, h, http.MethodPost, "/admin/tax-rates", at, map[string]any{"country": "KE", "tax_class": "standard", "rate": "0.1600", "name": "Kenya VAT"}); rr.Code != http.StatusCreated {
		t.Fatalf("create rate: %d (%s)", rr.Code, rr.Body.String())
	}

	// Calculate over a product line.
	var res struct {
		TaxTotal string `json:"tax_total"`
		Lines    []struct {
			TaxAmount string `json:"tax_amount"`
		} `json:"lines"`
	}
	rr := do(t, h, http.MethodPost, "/admin/tax/calculate", at, map[string]any{"country": "KE", "lines": []map[string]any{{"product_id": p.ID, "amount": "100.0000"}}})
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.TaxTotal != "16.0000" || res.Lines[0].TaxAmount != "16.0000" {
		t.Fatalf("calculate KE: %+v (%s)", res, rr.Body.String())
	}

	// A country with no configured rate → 0.
	var none struct {
		TaxTotal string `json:"tax_total"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodPost, "/admin/tax/calculate", at, map[string]any{"country": "US", "lines": []map[string]any{{"product_id": p.ID, "amount": "100.0000"}}}).Body.Bytes(), &none)
	if none.TaxTotal != "0" {
		t.Errorf("unconfigured country tax = %s, want 0", none.TaxTotal)
	}

	// List.
	var list struct {
		Items []any `json:"items"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodGet, "/admin/tax-rates", at, nil).Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Errorf("rates = %d, want 1", len(list.Items))
	}
}

func TestTaxAuth(t *testing.T) {
	h, issuer, _ := newServer(t)
	cust, _ := issuer.IssueStorefront(0, 1, 1)
	if rr := do(t, h, http.MethodPost, "/admin/tax-rates", cust, map[string]any{"country": "KE", "name": "x"}); rr.Code != http.StatusForbidden {
		t.Errorf("storefront token: want 403, got %d", rr.Code)
	}
}
