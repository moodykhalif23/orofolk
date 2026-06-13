package insights_test

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

func newServer(t *testing.T) (http.Handler, *auth.Issuer, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), issuer, pool
}

func do(t *testing.T, h http.Handler, method, path, tok string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func decode(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rr.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rr.Body.String())
	}
}

// seedOrder creates one non-cancelled order (with a line) for a fresh customer.
func seedOrder(t *testing.T, pool *pgxpool.Pool, org int64, grand string) {
	t.Helper()
	q := gen.New(pool)
	ctx := context.Background()
	cust, err := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: org, Name: "Acme Industrial", CreditLimit: "0"})
	if err != nil {
		t.Fatalf("customer: %v", err)
	}
	p, _ := q.CreateProduct(ctx, gen.CreateProductParams{
		OrganizationID: org, Sku: "IP1", Type: "simple", Name: "Insight Product",
		Slug: "ip1", Status: "active", Attributes: []byte("{}"), Unit: "each",
	})
	o, err := q.CreateOrder(ctx, gen.CreateOrderParams{
		OrganizationID: org, WebsiteID: 1, CustomerID: cust.ID, Currency: "USD",
		BillingAddress: []byte("{}"), ShippingAddress: []byte("{}"),
		Subtotal: grand, TaxTotal: "0", ShippingTotal: "0", GrandTotal: grand,
	})
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := q.AddOrderItem(ctx, gen.AddOrderItemParams{
		OrderID: o.ID, ProductID: p.ID, Sku: p.Sku, Name: p.Name,
		Quantity: "1", Unit: "each", UnitPrice: grand, TaxAmount: "0", RowTotal: grand,
	}); err != nil {
		t.Fatalf("order item: %v", err)
	}
}

func adminToken(t *testing.T, issuer *auth.Issuer, perms ...string) string {
	t.Helper()
	tok, _ := issuer.Issue("1", 1, "admin", perms)
	return tok
}

func TestInsightsMetricsAndDigest(t *testing.T) {
	h, issuer, pool := newServer(t)
	tok := adminToken(t, issuer, "report.view", "report.manage")
	seedOrder(t, pool, 1, "500.0000")

	// Live metrics reflect the seeded order immediately and flag the new account.
	var m struct {
		PeriodLabel string `json:"period_label"`
		Kpis        struct {
			Revenue      string `json:"revenue"`
			Orders       int64  `json:"orders"`
			NewCustomers int64  `json:"new_customers"`
		} `json:"kpis"`
		Anomalies []struct {
			Key      string `json:"key"`
			Severity string `json:"severity"`
		} `json:"anomalies"`
	}
	rr := do(t, h, http.MethodGet, "/admin/insights/metrics", tok, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("metrics: status %d (%s)", rr.Code, rr.Body.String())
	}
	decode(t, rr, &m)
	if m.Kpis.Orders != 1 || m.Kpis.Revenue != "500.0000" {
		t.Fatalf("metrics kpis: want 1 order / 500.0000, got %d / %s", m.Kpis.Orders, m.Kpis.Revenue)
	}
	if m.Kpis.NewCustomers != 1 {
		t.Errorf("new_customers: want 1, got %d", m.Kpis.NewCustomers)
	}
	if findKey(m.Anomalies, "new_customers") == "" {
		t.Errorf("expected a new_customers signal, got %+v", m.Anomalies)
	}

	// Generate a digest inline (no queue wired in the test server) — it persists.
	var genResp struct {
		Scheduled bool `json:"scheduled"`
		Digest    struct {
			ID        int64  `json:"id"`
			Narrative string `json:"narrative"`
			Source    string `json:"source"`
			Trigger   string `json:"trigger"`
		} `json:"digest"`
	}
	rr = do(t, h, http.MethodPost, "/admin/insights/generate", tok, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("generate: status %d (%s)", rr.Code, rr.Body.String())
	}
	decode(t, rr, &genResp)
	if genResp.Scheduled {
		t.Errorf("generate should run inline (no enqueuer), got scheduled=true")
	}
	if genResp.Digest.ID == 0 || genResp.Digest.Narrative == "" {
		t.Fatalf("expected a persisted digest with a narrative, got %+v", genResp.Digest)
	}
	if genResp.Digest.Source != "deterministic" || genResp.Digest.Trigger != "manual" {
		t.Errorf("digest source/trigger = %q/%q, want deterministic/manual", genResp.Digest.Source, genResp.Digest.Trigger)
	}

	// latest returns the digest just created.
	var latest struct {
		Digest *struct {
			ID int64 `json:"id"`
		} `json:"digest"`
	}
	decode(t, do(t, h, http.MethodGet, "/admin/insights/latest", tok, nil), &latest)
	if latest.Digest == nil || latest.Digest.ID != genResp.Digest.ID {
		t.Fatalf("latest: want digest %d, got %+v", genResp.Digest.ID, latest.Digest)
	}

	// list includes it.
	var list struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	decode(t, do(t, h, http.MethodGet, "/admin/insights", tok, nil), &list)
	if len(list.Items) == 0 || list.Items[0].ID != genResp.Digest.ID {
		t.Fatalf("list: want digest %d on top, got %+v", genResp.Digest.ID, list.Items)
	}
}

func TestInsightsAuth(t *testing.T) {
	h, issuer, _ := newServer(t)

	// Storefront token is the wrong audience for the admin surface.
	cust, _ := issuer.IssueStorefront(0, 1, 1)
	if rr := do(t, h, http.MethodGet, "/admin/insights/metrics", cust, nil); rr.Code != http.StatusForbidden {
		t.Errorf("storefront token on metrics: want 403, got %d", rr.Code)
	}

	// report.view can read but not trigger a generation (that needs report.manage).
	viewOnly := adminToken(t, issuer, "report.view")
	if rr := do(t, h, http.MethodGet, "/admin/insights/metrics", viewOnly, nil); rr.Code != http.StatusOK {
		t.Errorf("report.view on metrics: want 200, got %d", rr.Code)
	}
	if rr := do(t, h, http.MethodPost, "/admin/insights/generate", viewOnly, nil); rr.Code != http.StatusForbidden {
		t.Errorf("report.view on generate: want 403 (needs report.manage), got %d", rr.Code)
	}
}

func findKey(as []struct {
	Key      string `json:"key"`
	Severity string `json:"severity"`
}, key string) string {
	for _, a := range as {
		if a.Key == key {
			return a.Severity
		}
	}
	return ""
}
