package marketplace_test

import (
	"bytes"
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
	"b2bcommerce/internal/testsupport"
)

const testSecret = "test-secret-please-change"

func newServer(t *testing.T) (http.Handler, *pgxpool.Pool, *auth.Issuer) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(st, issuer), pool, issuer
}

func adminToken(t *testing.T, issuer *auth.Issuer, perms ...string) string {
	t.Helper()
	tok, err := issuer.Issue("1", 1, "admin", perms)
	if err != nil {
		t.Fatalf("issue admin token: %v", err)
	}
	return tok
}

func req(t *testing.T, h http.Handler, method, path, tok string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	return rr
}

// TestVendorCRUD covers the operator's vendor lifecycle: permission gating,
// create (with slug + commission), get, list, update, and a vendor-user with a
// login that then authenticates against the vendor portal end to end.
func TestVendorCRUD(t *testing.T) {
	h, _, issuer := newServer(t)
	manage := adminToken(t, issuer, "vendor.view", "vendor.manage")
	viewOnly := adminToken(t, issuer, "vendor.view")

	// Permission gating: view-only cannot create.
	if rr := req(t, h, http.MethodPost, "/admin/vendors", viewOnly, map[string]any{"name": "Nope"}); rr.Code != http.StatusForbidden {
		t.Fatalf("view-only create: want 403, got %d", rr.Code)
	}

	// Create.
	rr := req(t, h, http.MethodPost, "/admin/vendors", manage, map[string]any{
		"name": "Acme Supply Co", "commission_rate": "12.5", "contact_email": "ops@acme.test",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create vendor: want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	var v struct {
		ID             int64  `json:"id"`
		Slug           string `json:"slug"`
		Status         string `json:"status"`
		CommissionRate string `json:"commission_rate"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &v)
	if v.ID == 0 {
		t.Fatal("vendor id missing")
	}
	if v.Slug != "acme-supply-co" {
		t.Errorf("slug: want acme-supply-co, got %q", v.Slug)
	}
	if v.Status != "active" {
		t.Errorf("status default: want active, got %q", v.Status)
	}

	// Get.
	if rr := req(t, h, http.MethodGet, "/admin/vendors/"+itoa(v.ID), manage, nil); rr.Code != http.StatusOK {
		t.Errorf("get vendor: want 200, got %d", rr.Code)
	}

	// List shows it.
	rr = req(t, h, http.MethodGet, "/admin/vendors", viewOnly, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", rr.Code)
	}
	var list struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].ID != v.ID {
		t.Errorf("list: want 1 vendor %d, got %+v", v.ID, list.Items)
	}

	// Update: suspend + change commission.
	rr = req(t, h, http.MethodPut, "/admin/vendors/"+itoa(v.ID), manage, map[string]any{
		"name": "Acme Supply Co", "status": "suspended", "commission_rate": "15", "payout_terms_days": 45,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update vendor: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Create a vendor portal user and confirm it can log in.
	rr = req(t, h, http.MethodPost, "/admin/vendors/"+itoa(v.ID)+"/users", manage, map[string]any{
		"email": "owner@acme.test", "password": "vendor-pass-123", "full_name": "Acme Owner", "role": "admin",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create vendor user: want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	// A suspended vendor's users are blocked from logging in.
	if lr := req(t, h, http.MethodPost, "/vendor/auth/login", "", map[string]any{
		"email": "owner@acme.test", "password": "vendor-pass-123", "org_id": 1,
	}); lr.Code != http.StatusUnauthorized {
		t.Errorf("suspended vendor login: want 401, got %d", lr.Code)
	}

	// Reactivate, then the user can log in.
	if rr := req(t, h, http.MethodPut, "/admin/vendors/"+itoa(v.ID), manage, map[string]any{
		"name": "Acme Supply Co", "status": "active", "commission_rate": "15", "payout_terms_days": 45,
	}); rr.Code != http.StatusOK {
		t.Fatalf("reactivate: want 200, got %d", rr.Code)
	}
	if lr := req(t, h, http.MethodPost, "/vendor/auth/login", "", map[string]any{
		"email": "owner@acme.test", "password": "vendor-pass-123", "org_id": 1,
	}); lr.Code != http.StatusOK {
		t.Errorf("active vendor login: want 200, got %d (%s)", lr.Code, lr.Body.String())
	}

	// Soft delete.
	if rr := req(t, h, http.MethodDelete, "/admin/vendors/"+itoa(v.ID), manage, nil); rr.Code != http.StatusNoContent {
		t.Errorf("delete vendor: want 204, got %d", rr.Code)
	}
	if rr := req(t, h, http.MethodGet, "/admin/vendors/"+itoa(v.ID), manage, nil); rr.Code != http.StatusNotFound {
		t.Errorf("get deleted vendor: want 404, got %d", rr.Code)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
