package shipping_test

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

const testSecret = "ship-test-secret"

func newServer(t *testing.T) (http.Handler, *auth.Issuer, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), issuer, pool
}

func tok(t *testing.T, issuer *auth.Issuer) string {
	t.Helper()
	s, _ := issuer.Issue("1", 1, "admin", []string{"shipping.view", "shipping.manage"})
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

func TestShippingRatesQuoteAndLabel(t *testing.T) {
	h, issuer, pool := newServer(t)
	at := tok(t, issuer)
	q := gen.New(pool)
	ctx := context.Background()

	// Configure two services for KE; express is free over 100.
	for _, rate := range []map[string]any{
		{"country": "KE", "service": "standard", "amount": "10.0000"},
		{"country": "KE", "service": "express", "amount": "25.0000", "free_over": "100.0000"},
	} {
		if rr := do(t, h, http.MethodPost, "/admin/shipping-rates", at, rate); rr.Code != http.StatusCreated {
			t.Fatalf("create rate: %d (%s)", rr.Code, rr.Body.String())
		}
	}

	// Public rate quote (subtotal 150 → express free).
	var q150 struct {
		Quotes []struct {
			Service string `json:"service"`
			Amount  string `json:"amount"`
			Free    bool   `json:"free"`
		} `json:"quotes"`
	}
	rr := do(t, h, http.MethodPost, "/storefront/shipping/rates", "", map[string]any{"country": "KE", "subtotal": "150.0000"})
	_ = json.Unmarshal(rr.Body.Bytes(), &q150)
	if len(q150.Quotes) != 2 {
		t.Fatalf("quotes = %d, want 2 (%s)", len(q150.Quotes), rr.Body.String())
	}
	var sawFreeExpress bool
	for _, qt := range q150.Quotes {
		if qt.Service == "express" && qt.Free && qt.Amount == "0.0000" {
			sawFreeExpress = true
		}
	}
	if !sawFreeExpress {
		t.Errorf("express should be free over 100: %+v", q150.Quotes)
	}

	// Seed an order + shipment, then create a label and track it.
	cust, _ := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: 1, Name: "Ship Co", CreditLimit: "0"})
	o, err := q.CreateOrder(ctx, gen.CreateOrderParams{
		OrganizationID: 1, WebsiteID: 1, CustomerID: cust.ID, Currency: "USD",
		BillingAddress: []byte("{}"), ShippingAddress: []byte(`{"country":"KE"}`),
		Subtotal: "10", TaxTotal: "0", ShippingTotal: "0", GrandTotal: "10",
	})
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	sh, err := q.CreateShipment(ctx, gen.CreateShipmentParams{OrderID: o.ID})
	if err != nil {
		t.Fatalf("shipment: %v", err)
	}
	ss := strconv.FormatInt(sh.ID, 10)

	lbl := do(t, h, http.MethodPost, "/admin/shipments/"+ss+"/label", at, map[string]any{"service": "express"})
	if lbl.Code != http.StatusCreated {
		t.Fatalf("label: %d (%s)", lbl.Code, lbl.Body.String())
	}
	var label struct {
		TrackingNumber string `json:"tracking_number"`
	}
	_ = json.Unmarshal(lbl.Body.Bytes(), &label)
	if label.TrackingNumber == "" {
		t.Fatal("label has no tracking number")
	}

	// Tracking number persisted on the shipment.
	got, _ := q.GetShipment(ctx, sh.ID)
	if got.TrackingNumber == nil || *got.TrackingNumber != label.TrackingNumber {
		t.Errorf("tracking not persisted: %v", got.TrackingNumber)
	}

	// Track endpoint.
	var tr struct {
		Status string `json:"status"`
	}
	trr := do(t, h, http.MethodGet, "/admin/shipments/"+ss+"/track", at, nil)
	_ = json.Unmarshal(trr.Body.Bytes(), &tr)
	if tr.Status != "in_transit" {
		t.Errorf("track status = %q, want in_transit", tr.Status)
	}
}

func TestShippingAuth(t *testing.T) {
	h, issuer, _ := newServer(t)
	cust, _ := issuer.IssueStorefront(0, 1, 1)
	if rr := do(t, h, http.MethodPost, "/admin/shipping-rates", cust, map[string]any{"country": "KE", "service": "x"}); rr.Code != http.StatusForbidden {
		t.Errorf("storefront token: want 403, got %d", rr.Code)
	}
}
