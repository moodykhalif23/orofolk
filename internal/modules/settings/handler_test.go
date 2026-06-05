package settings_test

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

type resolveResp struct {
	Found bool            `json:"found"`
	Scope string          `json:"scope"`
	Value json.RawMessage `json:"value"`
}

func TestConfigCascadeResolution(t *testing.T) {
	h, issuer, pool := newServer(t)
	admin, _ := issuer.Issue("1", 1, "admin", []string{"settings.view", "settings.manage"})
	q := gen.New(pool)
	ctx := context.Background()

	grp, _ := q.CreateCustomerGroup(ctx, gen.CreateCustomerGroupParams{OrganizationID: 1, Name: "Wholesale"})
	cust, _ := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: 1, Name: "Acme", CreditLimit: "0", CustomerGroupID: &grp.ID})
	ws, _ := q.GetDefaultWebsite(ctx, 1)

	set := func(scope string, scopeID *int64, value string) {
		body := map[string]any{"scope": scope, "key": "min_order", "value": json.RawMessage(value)}
		if scopeID != nil {
			body["scope_id"] = *scopeID
		}
		if rr := do(t, h, http.MethodPut, "/admin/settings", admin, body); rr.Code != http.StatusOK {
			t.Fatalf("set %s: %d (%s)", scope, rr.Code, rr.Body.String())
		}
	}
	set("org", nil, "100")
	set("website", &ws.ID, "90")
	set("group", &grp.ID, "80")
	set("customer", &cust.ID, "70")

	resolve := func(query string) resolveResp {
		rr := do(t, h, http.MethodGet, "/admin/settings/resolve?key=min_order"+query, admin, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("resolve%s: %d (%s)", query, rr.Code, rr.Body.String())
		}
		var out resolveResp
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}

	cid := strconv.FormatInt(cust.ID, 10)
	gid := strconv.FormatInt(grp.ID, 10)
	wid := strconv.FormatInt(ws.ID, 10)

	if got := resolve("&customer_id=" + cid + "&group_id=" + gid + "&website_id=" + wid); got.Scope != "customer" || string(got.Value) != "70" {
		t.Errorf("full scope: want customer/70, got %s/%s", got.Scope, got.Value)
	}
	if got := resolve("&group_id=" + gid + "&website_id=" + wid); got.Scope != "group" || string(got.Value) != "80" {
		t.Errorf("group+website: want group/80, got %s/%s", got.Scope, got.Value)
	}
	if got := resolve("&website_id=" + wid); got.Scope != "website" || string(got.Value) != "90" {
		t.Errorf("website: want website/90, got %s/%s", got.Scope, got.Value)
	}
	if got := resolve(""); got.Scope != "org" || string(got.Value) != "100" {
		t.Errorf("org default: want org/100, got %s/%s", got.Scope, got.Value)
	}
}

func TestStorefrontResolveUsesBuyerScope(t *testing.T) {
	h, issuer, pool := newServer(t)
	admin, _ := issuer.Issue("1", 1, "admin", []string{"settings.view", "settings.manage"})
	q := gen.New(pool)
	ctx := context.Background()

	grp, _ := q.CreateCustomerGroup(ctx, gen.CreateCustomerGroupParams{OrganizationID: 1, Name: "Wholesale"})
	cust, _ := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: 1, Name: "Acme", CreditLimit: "0", CustomerGroupID: &grp.ID})

	do(t, h, http.MethodPut, "/admin/settings", admin, map[string]any{"scope": "org", "key": "free_ship", "value": json.RawMessage(`false`)})
	do(t, h, http.MethodPut, "/admin/settings", admin, map[string]any{"scope": "group", "scope_id": grp.ID, "key": "free_ship", "value": json.RawMessage(`true`)})

	custTok, _ := issuer.IssueStorefront(1, 1, cust.ID)
	rr := do(t, h, http.MethodGet, "/storefront/settings/free_ship", custTok, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("storefront resolve: %d (%s)", rr.Code, rr.Body.String())
	}
	var out resolveResp
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	// The buyer is in the wholesale group -> group override (true) beats org (false).
	if out.Scope != "group" || string(out.Value) != "true" {
		t.Errorf("buyer scope: want group/true, got %s/%s", out.Scope, out.Value)
	}
}

func TestSettingsValidationAndAuth(t *testing.T) {
	h, issuer, _ := newServer(t)
	admin, _ := issuer.Issue("1", 1, "admin", []string{"settings.view", "settings.manage"})

	// Bad scope.
	if rr := do(t, h, http.MethodPut, "/admin/settings", admin, map[string]any{"scope": "planet", "key": "k", "value": json.RawMessage(`1`)}); rr.Code != http.StatusBadRequest {
		t.Errorf("bad scope: want 400, got %d", rr.Code)
	}
	// org scope must not carry a scope_id.
	if rr := do(t, h, http.MethodPut, "/admin/settings", admin, map[string]any{"scope": "org", "scope_id": 5, "key": "k", "value": json.RawMessage(`1`)}); rr.Code != http.StatusBadRequest {
		t.Errorf("org with scope_id: want 400, got %d", rr.Code)
	}
	// Missing permission.
	viewless, _ := issuer.Issue("1", 1, "admin", []string{})
	if rr := do(t, h, http.MethodGet, "/admin/settings", viewless, nil); rr.Code != http.StatusForbidden {
		t.Errorf("no perm: want 403, got %d", rr.Code)
	}
}
