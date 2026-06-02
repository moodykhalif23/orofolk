package cart_test

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
	"b2bcommerce/internal/queue/jobs"
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

const custPassword = "buyer-pass-123"

type seed struct {
	customerID     int64
	email          string
	pricedPublicID string
	freePublicID   string // a product with no resolvable price
	productPriced  int64
}

// seedCustomer creates a customer + login + a customer-assigned price list with
// one priced product, then warms combined_prices via the recompute job.
func seedCustomer(t *testing.T, pool *pgxpool.Pool, name, email string) seed {
	t.Helper()
	q := gen.New(pool)
	ctx := context.Background()

	cust, err := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: 1, Name: name, CreditLimit: "0"})
	if err != nil {
		t.Fatalf("customer: %v", err)
	}
	hash, _ := auth.HashPassword(custPassword)
	if _, err := q.CreateCustomerUser(ctx, gen.CreateCustomerUserParams{
		CustomerID: cust.ID, Email: email, PasswordHash: hash, FullName: name + " Buyer", Role: "buyer",
	}); err != nil {
		t.Fatalf("customer user: %v", err)
	}

	priced, err := q.CreateProduct(ctx, gen.CreateProductParams{
		OrganizationID: 1, Sku: name + "-PRICED", Type: "simple", Name: "Priced", Slug: name + "-priced",
		Status: "active", Attributes: []byte("{}"), Unit: "each",
	})
	if err != nil {
		t.Fatalf("priced product: %v", err)
	}
	free, err := q.CreateProduct(ctx, gen.CreateProductParams{
		OrganizationID: 1, Sku: name + "-FREE", Type: "simple", Name: "Unpriced", Slug: name + "-free",
		Status: "active", Attributes: []byte("{}"), Unit: "each",
	})
	if err != nil {
		t.Fatalf("free product: %v", err)
	}

	list, err := q.CreatePriceList(ctx, gen.CreatePriceListParams{OrganizationID: 1, Name: name + " List", Currency: "USD", IsActive: true})
	if err != nil {
		t.Fatalf("price list: %v", err)
	}
	if _, err := q.UpsertPrice(ctx, gen.UpsertPriceParams{PriceListID: list.ID, ProductID: priced.ID, Unit: "each", MinQuantity: "1", Value: "10.0000"}); err != nil {
		t.Fatalf("price: %v", err)
	}
	if _, err := q.CreatePriceListAssignment(ctx, gen.CreatePriceListAssignmentParams{PriceListID: list.ID, CustomerID: &cust.ID}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	wid := int64(1)
	if err := jobs.RecomputeForCustomer(ctx, pool, jobs.RecomputeCombinedPricesArgs{CustomerID: cust.ID, WebsiteID: &wid, Currency: "USD"}); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	return seed{
		customerID: cust.ID, email: email,
		pricedPublicID: priced.PublicID.String(), freePublicID: free.PublicID.String(), productPriced: priced.ID,
	}
}

func login(t *testing.T, h http.Handler, email string) string {
	t.Helper()
	rr := do(t, h, http.MethodPost, "/storefront/auth/login", "", map[string]any{"email": email, "password": custPassword, "org_id": 1})
	if rr.Code != http.StatusOK {
		t.Fatalf("login %s: want 200, got %d (%s)", email, rr.Code, rr.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Token == "" {
		t.Fatal("empty token")
	}
	return resp.Token
}

type cartResp struct {
	Subtotal string `json:"subtotal"`
	Currency string `json:"currency"`
	Items    []struct {
		ID        int64  `json:"id"`
		UnitPrice string `json:"unit_price"`
		RowTotal  string `json:"row_total"`
		Quantity  string `json:"quantity"`
	} `json:"items"`
}

// ---- Auth + audience -----------------------------------------------------

func TestStorefrontLoginAndAudienceGate(t *testing.T) {
	h, issuer, pool := newServer(t)
	s := seedCustomer(t, pool, "acme", "buyer@acme.test")

	// Bad credentials.
	if rr := do(t, h, http.MethodPost, "/storefront/auth/login", "", map[string]any{"email": s.email, "password": "wrong", "org_id": 1}); rr.Code != http.StatusUnauthorized {
		t.Fatalf("bad creds: want 401, got %d", rr.Code)
	}
	// No token on a storefront route.
	if rr := do(t, h, http.MethodGet, "/storefront/cart", "", nil); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token: want 401, got %d", rr.Code)
	}
	// An admin-audience token must be rejected on storefront routes.
	adminTok, _ := issuer.Issue("1", 1, "admin", []string{"product.view"})
	if rr := do(t, h, http.MethodGet, "/storefront/cart", adminTok, nil); rr.Code != http.StatusForbidden {
		t.Fatalf("admin token on storefront: want 403, got %d", rr.Code)
	}
	// Proper storefront token works.
	tok := login(t, h, s.email)
	if rr := do(t, h, http.MethodGet, "/storefront/cart", tok, nil); rr.Code != http.StatusOK {
		t.Fatalf("storefront cart: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// ---- Cart pricing + totals -----------------------------------------------

func TestCartAddPriceAndTotals(t *testing.T) {
	h, _, pool := newServer(t)
	s := seedCustomer(t, pool, "acme", "buyer@acme.test")
	tok := login(t, h, s.email)

	// Add the priced product, qty 2 -> unit_price from combined_prices, subtotal 20.
	rr := do(t, h, http.MethodPost, "/storefront/cart/items", tok, map[string]any{"product_public_id": s.pricedPublicID, "quantity": "2"})
	if rr.Code != http.StatusOK {
		t.Fatalf("add priced: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var c cartResp
	_ = json.Unmarshal(rr.Body.Bytes(), &c)
	if len(c.Items) != 1 || c.Items[0].UnitPrice != "10.0000" {
		t.Fatalf("unit price snapshot: want 10.0000, got %+v", c.Items)
	}
	if c.Items[0].RowTotal != "20.0000" || c.Subtotal != "20.0000" {
		t.Errorf("totals: want row/subtotal 20.0000, got row=%s subtotal=%s", c.Items[0].RowTotal, c.Subtotal)
	}

	// Unpriced product -> 409 price-on-request.
	if rr := do(t, h, http.MethodPost, "/storefront/cart/items", tok, map[string]any{"product_public_id": s.freePublicID, "quantity": "1"}); rr.Code != http.StatusConflict {
		t.Fatalf("unpriced add: want 409, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Update quantity -> totals recompute.
	itemID := c.Items[0].ID
	upd := do(t, h, http.MethodPatch, "/storefront/cart/items/"+strconv.FormatInt(itemID, 10), tok, map[string]any{"quantity": "3"})
	if upd.Code != http.StatusOK {
		t.Fatalf("update qty: %d (%s)", upd.Code, upd.Body.String())
	}
	_ = json.Unmarshal(upd.Body.Bytes(), &c)
	if c.Subtotal != "30.0000" {
		t.Errorf("after qty=3: want subtotal 30.0000, got %s", c.Subtotal)
	}

	// Remove -> empty cart.
	del := do(t, h, http.MethodDelete, "/storefront/cart/items/"+strconv.FormatInt(itemID, 10), tok, nil)
	if del.Code != http.StatusOK {
		t.Fatalf("remove: %d", del.Code)
	}
	_ = json.Unmarshal(del.Body.Bytes(), &c)
	if len(c.Items) != 0 || c.Subtotal != "0.0000" {
		t.Errorf("after remove: want empty cart, got items=%d subtotal=%s", len(c.Items), c.Subtotal)
	}
}

func TestCartRevalidateOnPriceChange(t *testing.T) {
	h, _, pool := newServer(t)
	q := gen.New(pool)
	ctx := context.Background()
	s := seedCustomer(t, pool, "acme", "buyer@acme.test")
	tok := login(t, h, s.email)

	// Add at the seeded price (10).
	do(t, h, http.MethodPost, "/storefront/cart/items", tok, map[string]any{"product_public_id": s.pricedPublicID, "quantity": "1"})

	// Change the price to 8 and rewarm the cache.
	lists, _ := q.ListPriceLists(ctx, 1)
	var listID int64
	for _, l := range lists {
		if l.Currency == "USD" {
			listID = l.ID
		}
	}
	if _, err := q.UpsertPrice(ctx, gen.UpsertPriceParams{PriceListID: listID, ProductID: s.productPriced, Unit: "each", MinQuantity: "1", Value: "8.0000"}); err != nil {
		t.Fatalf("reprice: %v", err)
	}
	wid := int64(1)
	if err := jobs.RecomputeForCustomer(ctx, pool, jobs.RecomputeCombinedPricesArgs{CustomerID: s.customerID, WebsiteID: &wid, Currency: "USD"}); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	// Revalidate picks up the drift.
	rr := do(t, h, http.MethodPost, "/storefront/cart/revalidate", tok, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("revalidate: %d (%s)", rr.Code, rr.Body.String())
	}
	var res struct {
		Changed []struct {
			OldPrice string `json:"old_price"`
			NewPrice string `json:"new_price"`
		} `json:"changed"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if len(res.Changed) != 1 || res.Changed[0].NewPrice != "8.0000" || res.Changed[0].OldPrice != "10.0000" {
		t.Fatalf("revalidate drift: want 10->8, got %+v", res.Changed)
	}

	// Cart now reflects the new price.
	cartRR := do(t, h, http.MethodGet, "/storefront/cart", tok, nil)
	var c cartResp
	_ = json.Unmarshal(cartRR.Body.Bytes(), &c)
	if c.Subtotal != "8.0000" {
		t.Errorf("post-revalidate subtotal: want 8.0000, got %s", c.Subtotal)
	}
}

// ---- Shopping lists ------------------------------------------------------

func TestShoppingListConvertAndDefault(t *testing.T) {
	h, _, pool := newServer(t)
	s := seedCustomer(t, pool, "acme", "buyer@acme.test")
	tok := login(t, h, s.email)

	// First default list ok.
	l1 := do(t, h, http.MethodPost, "/storefront/shopping-lists", tok, map[string]any{"name": "Reorder", "is_default": true})
	if l1.Code != http.StatusCreated {
		t.Fatalf("create list: %d (%s)", l1.Code, l1.Body.String())
	}
	var list gen.ShoppingList
	_ = json.Unmarshal(l1.Body.Bytes(), &list)

	// Second default list rejected by the partial unique index.
	if dup := do(t, h, http.MethodPost, "/storefront/shopping-lists", tok, map[string]any{"name": "Other", "is_default": true}); dup.Code != http.StatusConflict {
		t.Errorf("second default list: want 409, got %d", dup.Code)
	}

	// Add a priced + an unpriced item.
	base := "/storefront/shopping-lists/" + strconv.FormatInt(list.ID, 10) + "/items"
	do(t, h, http.MethodPost, base, tok, map[string]any{"product_public_id": s.pricedPublicID, "quantity": "2"})
	do(t, h, http.MethodPost, base, tok, map[string]any{"product_public_id": s.freePublicID, "quantity": "1"})

	// Convert -> priced item lands in the cart, unpriced is skipped.
	conv := do(t, h, http.MethodPost, "/storefront/shopping-lists/"+strconv.FormatInt(list.ID, 10)+"/convert-to-cart", tok, nil)
	if conv.Code != http.StatusOK {
		t.Fatalf("convert: %d (%s)", conv.Code, conv.Body.String())
	}
	var cres struct {
		Skipped []int64 `json:"skipped_product_ids"`
	}
	_ = json.Unmarshal(conv.Body.Bytes(), &cres)
	if len(cres.Skipped) != 1 {
		t.Errorf("convert: want 1 skipped (unpriced), got %v", cres.Skipped)
	}
	cartRR := do(t, h, http.MethodGet, "/storefront/cart", tok, nil)
	var c cartResp
	_ = json.Unmarshal(cartRR.Body.Bytes(), &c)
	if len(c.Items) != 1 || c.Subtotal != "20.0000" {
		t.Errorf("converted cart: want 1 item subtotal 20.0000, got items=%d subtotal=%s", len(c.Items), c.Subtotal)
	}
}

// ---- Isolation -----------------------------------------------------------

func TestCartCustomerIsolation(t *testing.T) {
	h, _, pool := newServer(t)
	a := seedCustomer(t, pool, "acme", "buyer@acme.test")
	b := seedCustomer(t, pool, "beta", "buyer@beta.test")

	tokA := login(t, h, a.email)
	tokB := login(t, h, b.email)

	// A adds an item.
	if rr := do(t, h, http.MethodPost, "/storefront/cart/items", tokA, map[string]any{"product_public_id": a.pricedPublicID, "quantity": "1"}); rr.Code != http.StatusOK {
		t.Fatalf("A add: %d (%s)", rr.Code, rr.Body.String())
	}
	// B's cart is independent and empty.
	rr := do(t, h, http.MethodGet, "/storefront/cart", tokB, nil)
	var c cartResp
	_ = json.Unmarshal(rr.Body.Bytes(), &c)
	if len(c.Items) != 0 {
		t.Errorf("isolation: customer B should not see A's cart items, got %d", len(c.Items))
	}
}
