package account_test

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
const custPassword = "buyer-pass-123"

func newServer(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), pool
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

// seedCustomer creates a customer company + one buyer login.
func seedCustomer(t *testing.T, pool *pgxpool.Pool, name, email string) int64 {
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
	return cust.ID
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
	return resp.Token
}

func TestAccountAddressesCreateListAndIsolation(t *testing.T) {
	h, pool := newServer(t)
	seedCustomer(t, pool, "acme", "buyer@acme.test")
	bID := seedCustomer(t, pool, "beta", "buyer@beta.test")
	_ = bID
	tokA := login(t, h, "buyer@acme.test")
	tokB := login(t, h, "buyer@beta.test")

	// A creates a shipping address.
	cr := do(t, h, http.MethodPost, "/storefront/account/addresses", tokA, map[string]any{
		"type": "shipping", "is_default": true, "line1": "1 Market St", "city": "SF", "country": "US",
	})
	if cr.Code != http.StatusCreated {
		t.Fatalf("create address: want 201, got %d (%s)", cr.Code, cr.Body.String())
	}

	// A sees exactly their one address.
	la := do(t, h, http.MethodGet, "/storefront/account/addresses", tokA, nil)
	var ra struct {
		Items []gen.CustomerAddress `json:"items"`
	}
	_ = json.Unmarshal(la.Body.Bytes(), &ra)
	if len(ra.Items) != 1 || ra.Items[0].City != "SF" {
		t.Fatalf("A addresses: want 1 (SF), got %+v", ra.Items)
	}

	// B's address list is independent and empty — no cross-company leakage.
	lb := do(t, h, http.MethodGet, "/storefront/account/addresses", tokB, nil)
	var rb struct {
		Items []gen.CustomerAddress `json:"items"`
	}
	_ = json.Unmarshal(lb.Body.Bytes(), &rb)
	if len(rb.Items) != 0 {
		t.Errorf("B addresses: want 0, got %d", len(rb.Items))
	}
}

func TestAccountAddressValidation(t *testing.T) {
	h, pool := newServer(t)
	seedCustomer(t, pool, "acme", "buyer@acme.test")
	tok := login(t, h, "buyer@acme.test")

	// Missing line1/city/country.
	if rr := do(t, h, http.MethodPost, "/storefront/account/addresses", tok, map[string]any{"type": "shipping"}); rr.Code != http.StatusBadRequest {
		t.Errorf("invalid address: want 400, got %d", rr.Code)
	}
	// Bad type.
	if rr := do(t, h, http.MethodPost, "/storefront/account/addresses", tok, map[string]any{
		"type": "warehouse", "line1": "x", "city": "y", "country": "US",
	}); rr.Code != http.StatusBadRequest {
		t.Errorf("bad type: want 400, got %d", rr.Code)
	}
}

func TestAccountRequiresStorefrontToken(t *testing.T) {
	h, _ := newServer(t)
	if rr := do(t, h, http.MethodGet, "/storefront/account/addresses", "", nil); rr.Code != http.StatusUnauthorized {
		t.Errorf("no token: want 401, got %d", rr.Code)
	}
}
