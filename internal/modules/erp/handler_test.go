package erp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/auth"
	erpconn "b2bcommerce/internal/erp"
	"b2bcommerce/internal/server"
	"b2bcommerce/internal/store"
	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
)

const testSecret = "erp-test-secret"

func newServer(t *testing.T) (http.Handler, *auth.Issuer, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), issuer, pool
}

func tok(t *testing.T, issuer *auth.Issuer) string {
	t.Helper()
	s, _ := issuer.Issue("1", 1, "admin", []string{"erp.view", "erp.manage"})
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

func TestERPOutboundSweepIdempotent(t *testing.T) {
	h, issuer, pool := newServer(t)
	at := tok(t, issuer)
	q := gen.New(pool)
	ctx := context.Background()

	// A fake ERP that accepts pushes and returns an external id.
	var mu sync.Mutex
	received := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		received++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"external_id":"ERP-OK"}`))
	}))
	defer srv.Close()

	// Register the connection.
	var conn struct {
		ID int64 `json:"id"`
	}
	cr := do(t, h, http.MethodPost, "/admin/erp/connections", at, map[string]any{"provider": "generic_webhook", "endpoint": srv.URL, "secret": "topsecret"})
	if cr.Code != http.StatusCreated {
		t.Fatalf("create connection: %d (%s)", cr.Code, cr.Body.String())
	}
	_ = json.Unmarshal(cr.Body.Bytes(), &conn)

	// Seed a confirmed order + an issued invoice.
	cust, _ := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: 1, Name: "ERP Co", CreditLimit: "0"})
	o, _ := q.CreateOrder(ctx, gen.CreateOrderParams{
		OrganizationID: 1, WebsiteID: 1, CustomerID: cust.ID, Currency: "USD",
		BillingAddress: []byte("{}"), ShippingAddress: []byte("{}"),
		Subtotal: "100", TaxTotal: "0", ShippingTotal: "0", GrandTotal: "100",
	})
	if _, err := q.SetOrderStatus(ctx, gen.SetOrderStatusParams{ID: o.ID, Status: "confirmed"}); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if _, err := q.CreateInvoice(ctx, gen.CreateInvoiceParams{
		OrderID: o.ID, CustomerID: cust.ID, Currency: "USD", Subtotal: "100", TaxTotal: "0", GrandTotal: "100",
		IssuedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		t.Fatalf("invoice: %v", err)
	}

	// Sweep → order + invoice pushed.
	cs := strconv.FormatInt(conn.ID, 10)
	var res struct {
		Synced int `json:"synced"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodPost, "/admin/erp/connections/"+cs+"/sync", at, nil).Body.Bytes(), &res)
	if res.Synced != 2 {
		t.Fatalf("first sweep synced = %d, want 2", res.Synced)
	}

	// Re-sweep is idempotent: nothing new (external_refs already recorded).
	var res2 struct {
		Synced int `json:"synced"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodPost, "/admin/erp/connections/"+cs+"/sync", at, nil).Body.Bytes(), &res2)
	if res2.Synced != 0 {
		t.Errorf("second sweep synced = %d, want 0 (idempotent)", res2.Synced)
	}
	mu.Lock()
	got := received
	mu.Unlock()
	if got != 2 {
		t.Errorf("ERP received %d pushes, want 2", got)
	}

	// Sync log has two outbound 'sent' rows.
	var logs struct {
		Items []struct {
			Direction string `json:"direction"`
			Status    string `json:"status"`
		} `json:"items"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodGet, "/admin/erp/sync-logs", at, nil).Body.Bytes(), &logs)
	sent := 0
	for _, l := range logs.Items {
		if l.Direction == "outbound" && l.Status == "sent" {
			sent++
		}
	}
	if sent != 2 {
		t.Errorf("outbound sent logs = %d, want 2", sent)
	}
}

func TestERPInboundInventoryWebhook(t *testing.T) {
	h, issuer, pool := newServer(t)
	at := tok(t, issuer)
	q := gen.New(pool)
	ctx := context.Background()

	// Connection with a secret (so the webhook is signed).
	var conn struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(do(t, h, http.MethodPost, "/admin/erp/connections", at, map[string]any{"provider": "generic_webhook", "secret": "wsecret"}).Body.Bytes(), &conn)

	// Seed a warehouse + product the ERP will report stock for.
	if _, err := q.CreateWarehouse(ctx, gen.CreateWarehouseParams{OrganizationID: 1, Name: "Main"}); err != nil {
		t.Fatalf("warehouse: %v", err)
	}
	prod, _ := q.CreateProduct(ctx, gen.CreateProductParams{OrganizationID: 1, Sku: "ERP-SKU", Type: "simple", Name: "ERP Item", Slug: "erp-item", Status: "active", Attributes: []byte("{}"), Unit: "each"})

	body, _ := json.Marshal(map[string]any{"event_id": "evt-1", "entity_type": "inventory", "sku": "ERP-SKU", "quantity_on_hand": "500.0000"})

	post := func(sig string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/erp/"+strconv.FormatInt(conn.ID, 10), bytes.NewReader(body))
		req.Header.Set(erpconn.SignatureHeader, sig)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	// Bad signature → 401.
	if rr := post("deadbeef"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("bad signature: want 401, got %d", rr.Code)
	}

	// Valid signature → processed, stock set to 500.
	good := erpconn.Sign("wsecret", body)
	if rr := post(good); rr.Code != http.StatusOK {
		t.Fatalf("inbound: %d (%s)", rr.Code, rr.Body.String())
	}
	var onHand string
	if err := pool.QueryRow(ctx, `SELECT quantity_on_hand::text FROM inventory_levels WHERE product_id=$1`, prod.ID).Scan(&onHand); err != nil {
		t.Fatalf("read stock: %v", err)
	}
	if onHand != "500.0000" {
		t.Errorf("on-hand = %s, want 500.0000", onHand)
	}

	// Replaying the same event_id is a no-op (deduped).
	var dup struct {
		Duplicate bool `json:"duplicate"`
	}
	rr := post(good)
	_ = json.Unmarshal(rr.Body.Bytes(), &dup)
	if rr.Code != http.StatusOK || !dup.Duplicate {
		t.Errorf("replay: want 200 duplicate, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestERPAuth(t *testing.T) {
	h, issuer, _ := newServer(t)
	cust, _ := issuer.IssueStorefront(0, 1, 1)
	if rr := do(t, h, http.MethodGet, "/admin/erp/connections", cust, nil); rr.Code != http.StatusForbidden {
		t.Errorf("storefront token: want 403, got %d", rr.Code)
	}
}
