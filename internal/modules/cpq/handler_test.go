package cpq_test

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

const testSecret = "cpq-test-secret"

func newServer(t *testing.T) (http.Handler, *auth.Issuer, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), issuer, pool
}

func adminToken(t *testing.T, issuer *auth.Issuer) string {
	t.Helper()
	tok, _ := issuer.Issue("1", 1, "admin", []string{"product.view", "product.manage", "quote.view", "quote.manage"})
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

func decodeID(t *testing.T, rr *httptest.ResponseRecorder) int64 {
	t.Helper()
	var m struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &m)
	return m.ID
}

// seedConfigurableProduct creates a configurable product and returns its id +
// public id.
func seedProduct(t *testing.T, pool *pgxpool.Pool) (int64, string) {
	t.Helper()
	p, err := gen.New(pool).CreateProduct(context.Background(), gen.CreateProductParams{
		OrganizationID: 1, Sku: "LAPTOP", Type: "configurable", Name: "Laptop",
		Slug: "laptop", Status: "active", Attributes: []byte("{}"), Unit: "each",
	})
	if err != nil {
		t.Fatalf("product: %v", err)
	}
	return p.ID, p.PublicID.String()
}

func TestCPQConfiguratorAndQuoteLine(t *testing.T) {
	h, issuer, pool := newServer(t)
	tok := adminToken(t, issuer)
	pid, publicID := seedProduct(t, pool)
	ps := strconv.FormatInt(pid, 10)

	// 1. Base config.
	if rr := do(t, h, http.MethodPut, "/admin/products/"+ps+"/config", tok, map[string]any{"base_price": "1000.0000", "currency": "USD"}); rr.Code != http.StatusOK {
		t.Fatalf("config: %d (%s)", rr.Code, rr.Body.String())
	}
	// 2. Option group "ram" (required) + options.
	gid := decodeID(t, do(t, h, http.MethodPost, "/admin/products/"+ps+"/option-groups", tok, map[string]any{"code": "ram", "name": "RAM", "required": true, "min_select": 1, "max_select": 1}))
	if gid == 0 {
		t.Fatal("group not created")
	}
	gs := strconv.FormatInt(gid, 10)
	opt8 := decodeID(t, do(t, h, http.MethodPost, "/admin/option-groups/"+gs+"/options", tok, map[string]any{"code": "8gb", "name": "8 GB", "price_delta": "0"}))
	opt16 := decodeID(t, do(t, h, http.MethodPost, "/admin/option-groups/"+gs+"/options", tok, map[string]any{"code": "16gb", "name": "16 GB", "price_delta": "250.0000"}))
	if opt8 == 0 || opt16 == 0 {
		t.Fatal("options not created")
	}

	// 3. Get the assembled config.
	cfg := do(t, h, http.MethodGet, "/admin/products/"+ps+"/config", tok, nil)
	if cfg.Code != http.StatusOK || !contains(cfg.Body.String(), "\"ram\"") {
		t.Fatalf("get config: %d (%s)", cfg.Code, cfg.Body.String())
	}

	// 4. Storefront configure: 16GB → 1250; missing required group → invalid.
	type confResult struct {
		Valid     bool     `json:"valid"`
		UnitPrice string   `json:"unit_price"`
		Errors    []string `json:"errors"`
	}
	var ok confResult
	rr := do(t, h, http.MethodPost, "/storefront/products/"+publicID+"/configure", "", map[string]any{"selections": []int64{opt16}})
	_ = json.Unmarshal(rr.Body.Bytes(), &ok)
	if !ok.Valid || ok.UnitPrice != "1250.0000" {
		t.Fatalf("configure valid: %+v (%s)", ok, rr.Body.String())
	}
	var bad confResult
	_ = json.Unmarshal(do(t, h, http.MethodPost, "/storefront/products/"+publicID+"/configure", "", map[string]any{"selections": []int64{}}).Body.Bytes(), &bad)
	if bad.Valid {
		t.Error("empty selection should be invalid (ram required)")
	}

	// 5. Configured quote line.
	q, err := gen.New(pool).CreateQuote(context.Background(), gen.CreateQuoteParams{
		OrganizationID: 1, WebsiteID: 1, CustomerID: seedCustomer(t, pool), Currency: "USD",
	})
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	qs := strconv.FormatInt(q.ID, 10)

	// Invalid config is rejected (422).
	if rr := do(t, h, http.MethodPost, "/admin/quotes/"+qs+"/configured-lines", tok, map[string]any{"product_id": pid, "quantity": "1", "selections": []int64{}}); rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid configured line: want 422, got %d", rr.Code)
	}

	// Valid config: qty 2 × 1250 = 2500 line, quote subtotal updated.
	add := do(t, h, http.MethodPost, "/admin/quotes/"+qs+"/configured-lines", tok, map[string]any{"product_id": pid, "quantity": "2", "selections": []int64{opt16}})
	if add.Code != http.StatusCreated {
		t.Fatalf("configured line: %d (%s)", add.Code, add.Body.String())
	}
	var res struct {
		Item struct {
			RowTotal      string          `json:"row_total"`
			UnitPrice     string          `json:"unit_price"`
			Configuration json.RawMessage `json:"configuration"`
		} `json:"item"`
		QuoteSubtotal string `json:"quote_subtotal"`
	}
	_ = json.Unmarshal(add.Body.Bytes(), &res)
	if res.Item.UnitPrice != "1250.0000" || res.Item.RowTotal != "2500.0000" {
		t.Errorf("line pricing: unit=%s row=%s", res.Item.UnitPrice, res.Item.RowTotal)
	}
	if res.QuoteSubtotal != "2500.0000" {
		t.Errorf("quote subtotal = %s, want 2500.0000", res.QuoteSubtotal)
	}
	if len(res.Item.Configuration) == 0 || string(res.Item.Configuration) == "null" {
		t.Error("configured line should persist the configuration")
	}
}

func TestCPQAuth(t *testing.T) {
	h, issuer, pool := newServer(t)
	pid, _ := seedProduct(t, pool)
	ps := strconv.FormatInt(pid, 10)
	// storefront token cannot manage config.
	cust, _ := issuer.IssueStorefront(0, 1, 1)
	if rr := do(t, h, http.MethodPut, "/admin/products/"+ps+"/config", cust, map[string]any{"base_price": "1"}); rr.Code != http.StatusForbidden {
		t.Errorf("storefront config write: want 403, got %d", rr.Code)
	}
}

func seedCustomer(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	c, err := gen.New(pool).CreateCustomer(context.Background(), gen.CreateCustomerParams{OrganizationID: 1, Name: "Cfg Buyer", CreditLimit: "0"})
	if err != nil {
		t.Fatalf("customer: %v", err)
	}
	return c.ID
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
