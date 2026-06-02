package inventory_test

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
	"b2bcommerce/internal/inventory"
	"b2bcommerce/internal/money"
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

func adminToken(t *testing.T, issuer *auth.Issuer) string {
	t.Helper()
	tok, _ := issuer.Issue("1", 1, "admin", []string{
		"inventory.view", "inventory.manage", "order.view", "order.manage",
		"shipment.view", "shipment.manage",
	})
	return tok
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

func mkProduct(t *testing.T, q *gen.Queries, sku string) int64 {
	t.Helper()
	p, err := q.CreateProduct(context.Background(), gen.CreateProductParams{
		OrganizationID: 1, Sku: sku, Type: "simple", Name: sku, Slug: sku, Status: "active", Attributes: []byte("{}"), Unit: "each",
	})
	if err != nil {
		t.Fatalf("product %s: %v", sku, err)
	}
	return p.ID
}

// ---- Manual movements + ATP ----------------------------------------------

func TestReceiptAdjustmentAndATP(t *testing.T) {
	h, issuer, pool := newServer(t)
	tok := adminToken(t, issuer)
	q := gen.New(pool)
	prod := mkProduct(t, q, "WIDGET")

	whResp := do(t, h, http.MethodPost, "/admin/warehouses", tok, map[string]any{"name": "Main"})
	if whResp.Code != http.StatusCreated {
		t.Fatalf("warehouse: %d (%s)", whResp.Code, whResp.Body.String())
	}
	var wh struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(whResp.Body.Bytes(), &wh)

	// Receipt +100, adjustment -10 -> on_hand 90.
	if rr := do(t, h, http.MethodPost, "/admin/inventory/adjustments", tok, map[string]any{
		"product_id": prod, "warehouse_id": wh.ID, "type": "receipt", "quantity": "100",
	}); rr.Code != http.StatusCreated {
		t.Fatalf("receipt: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := do(t, h, http.MethodPost, "/admin/inventory/adjustments", tok, map[string]any{
		"product_id": prod, "warehouse_id": wh.ID, "type": "adjustment", "quantity": "-10",
	}); rr.Code != http.StatusCreated {
		t.Fatalf("adjustment: %d (%s)", rr.Code, rr.Body.String())
	}

	// ATP = 90.
	rr := do(t, h, http.MethodGet, "/admin/inventory/atp?product_ids="+strconv.FormatInt(prod, 10), tok, nil)
	var atp struct {
		Available map[string]string `json:"available"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &atp)
	if atp.Available[strconv.FormatInt(prod, 10)] != "90.0000" {
		t.Errorf("ATP: want 90.0000, got %v", atp.Available)
	}

	// Movement ledger has both entries.
	mv := do(t, h, http.MethodGet, "/admin/inventory/movements?product_id="+strconv.FormatInt(prod, 10)+"&warehouse_id="+strconv.FormatInt(wh.ID, 10), tok, nil)
	var movements struct {
		Items []any `json:"items"`
	}
	_ = json.Unmarshal(mv.Body.Bytes(), &movements)
	if len(movements.Items) != 2 {
		t.Errorf("movement ledger: want 2, got %d", len(movements.Items))
	}
}

// ---- Reservation rules (domain level) ------------------------------------

func TestReserveRespectsAvailabilityAndBackorder(t *testing.T) {
	_, _, pool := newServer(t)
	q := gen.New(pool)
	ctx := context.Background()

	wh, _ := q.CreateWarehouse(ctx, gen.CreateWarehouseParams{OrganizationID: 1, Name: "Main"})
	prod := mkProduct(t, q, "WIDGET")
	// Stock 5 on hand, no backorder.
	_ = q.EnsureInventoryLevel(ctx, gen.EnsureInventoryLevelParams{ProductID: prod, WarehouseID: wh.ID})
	_, _ = q.AdjustInventoryLevel(ctx, gen.AdjustInventoryLevelParams{ProductID: prod, WarehouseID: wh.ID, Column3: "5", Column4: "0"})

	// Order for 8 (> 5) -> reserve fails.
	order, _ := q.CreateOrder(ctx, gen.CreateOrderParams{
		OrganizationID: 1, WebsiteID: 1, CustomerID: mkCustomer(t, q), Currency: "USD",
		BillingAddress: []byte("{}"), ShippingAddress: []byte("{}"), Subtotal: "0", TaxTotal: "0", ShippingTotal: "0", GrandTotal: "0",
	})
	_, _ = q.AddOrderItem(ctx, gen.AddOrderItemParams{OrderID: order.ID, ProductID: prod, Sku: "WIDGET", Name: "WIDGET", Quantity: "8", Unit: "each", UnitPrice: "1", TaxAmount: "0", RowTotal: "8"})

	if err := reserve(t, pool, order.ID); err == nil {
		t.Fatal("expected insufficient stock error for qty 8 vs available 5")
	}

	// Allow backorder -> now it reserves.
	_, _ = q.SetInventoryLevelConfig(ctx, gen.SetInventoryLevelConfigParams{ProductID: prod, WarehouseID: wh.ID, AllowBackorder: true})
	if err := reserve(t, pool, order.ID); err != nil {
		t.Fatalf("backorder reserve should succeed: %v", err)
	}
	lvl, _ := q.GetInventoryLevel(ctx, gen.GetInventoryLevelParams{ProductID: prod, WarehouseID: wh.ID})
	if lvl.QuantityReserved != "8.0000" {
		t.Errorf("reserved after backorder: want 8.0000, got %s", lvl.QuantityReserved)
	}
}

// ---- End-to-end: confirm reserves, ship fulfils --------------------------

func TestConfirmReservesAndShipFulfils(t *testing.T) {
	h, issuer, pool := newServer(t)
	tok := adminToken(t, issuer)
	q := gen.New(pool)
	ctx := context.Background()

	wh, _ := q.CreateWarehouse(ctx, gen.CreateWarehouseParams{OrganizationID: 1, Name: "Main"})
	prod := mkProduct(t, q, "WIDGET")
	_ = q.EnsureInventoryLevel(ctx, gen.EnsureInventoryLevelParams{ProductID: prod, WarehouseID: wh.ID})
	_, _ = q.AdjustInventoryLevel(ctx, gen.AdjustInventoryLevelParams{ProductID: prod, WarehouseID: wh.ID, Column3: "10", Column4: "0"})

	custID := mkCustomer(t, q)
	order, _ := q.CreateOrder(ctx, gen.CreateOrderParams{
		OrganizationID: 1, WebsiteID: 1, CustomerID: custID, Currency: "USD",
		BillingAddress: []byte("{}"), ShippingAddress: []byte("{}"), Subtotal: "0", TaxTotal: "0", ShippingTotal: "0", GrandTotal: "0",
	})
	oi, _ := q.AddOrderItem(ctx, gen.AddOrderItemParams{OrderID: order.ID, ProductID: prod, Sku: "WIDGET", Name: "WIDGET", Quantity: "3", Unit: "each", UnitPrice: "1", TaxAmount: "0", RowTotal: "3"})

	// Confirm the order -> reserves 3.
	oid := strconv.FormatInt(order.ID, 10)
	if rr := do(t, h, http.MethodPatch, "/admin/orders/"+oid+"/status", tok, map[string]any{"status": "confirmed"}); rr.Code != http.StatusOK {
		t.Fatalf("confirm: %d (%s)", rr.Code, rr.Body.String())
	}
	lvl, _ := q.GetInventoryLevel(ctx, gen.GetInventoryLevelParams{ProductID: prod, WarehouseID: wh.ID})
	if !eq(lvl.QuantityReserved, "3") || !eq(lvl.QuantityOnHand, "10") {
		t.Fatalf("after confirm: want reserved 3 on_hand 10, got reserved %s on_hand %s", lvl.QuantityReserved, lvl.QuantityOnHand)
	}

	// Ship the line -> fulfilment drops on_hand and reserved by 3.
	shResp := do(t, h, http.MethodPost, "/admin/orders/"+oid+"/shipments", tok, map[string]any{
		"items": []map[string]any{{"order_item_id": oi.ID, "quantity": "3"}},
	})
	var sh struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(shResp.Body.Bytes(), &sh)
	if rr := do(t, h, http.MethodPatch, "/admin/shipments/"+strconv.FormatInt(sh.ID, 10)+"/status", tok, map[string]any{"status": "shipped"}); rr.Code != http.StatusOK {
		t.Fatalf("ship: %d (%s)", rr.Code, rr.Body.String())
	}
	lvl, _ = q.GetInventoryLevel(ctx, gen.GetInventoryLevelParams{ProductID: prod, WarehouseID: wh.ID})
	if !eq(lvl.QuantityOnHand, "7") || !eq(lvl.QuantityReserved, "0") {
		t.Errorf("after ship: want on_hand 7 reserved 0, got on_hand %s reserved %s", lvl.QuantityOnHand, lvl.QuantityReserved)
	}
}

// eq compares two decimal strings for numeric equality (scale-insensitive).
func eq(a, b string) bool {
	c, err := money.Cmp(a, b)
	return err == nil && c == 0
}

// ---- auth -----------------------------------------------------------------

func TestInventoryAuth(t *testing.T) {
	h, issuer, _ := newServer(t)
	// No permission -> 403.
	noPerm, _ := issuer.Issue("1", 1, "admin", []string{})
	if rr := do(t, h, http.MethodGet, "/admin/warehouses", noPerm, nil); rr.Code != http.StatusForbidden {
		t.Errorf("missing perm: want 403, got %d", rr.Code)
	}
	// Storefront token -> 403 (wrong audience).
	sf, _ := issuer.IssueStorefront(1, 1, 1)
	if rr := do(t, h, http.MethodGet, "/admin/warehouses", sf, nil); rr.Code != http.StatusForbidden {
		t.Errorf("storefront token: want 403, got %d", rr.Code)
	}
}

// ---- helpers --------------------------------------------------------------

func mkCustomer(t *testing.T, q *gen.Queries) int64 {
	t.Helper()
	c, err := q.CreateCustomer(context.Background(), gen.CreateCustomerParams{OrganizationID: 1, Name: "Cust", CreditLimit: "0"})
	if err != nil {
		t.Fatalf("customer: %v", err)
	}
	return c.ID
}

// reserve runs ReserveForOrder in its own transaction (commits on success).
func reserve(t *testing.T, pool *pgxpool.Pool, orderID int64) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := inventory.ReserveForOrder(ctx, gen.New(tx), 1, orderID, "test"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
