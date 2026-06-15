package objects_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/server"
	"b2bcommerce/internal/store"
	"b2bcommerce/internal/testsupport"
)

const testSecret = "test-secret-please-change"

func newServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	tok, err := issuer.Issue("1", 1, "admin", []string{"object.view", "object.manage"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return server.New(st, issuer), tok
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

// TestObjectModelingFlow certifies the whole slice against real Postgres through
// the HTTP stack: define a custom type + field rule, then create/list records,
// with the Phase-1 validation engine rejecting a bad value and the delete guard
// protecting a type that still has records.
func TestObjectModelingFlow(t *testing.T) {
	h, token := newServer(t)

	// 1. Define a "supplier" object type.
	rr := do(t, h, http.MethodPost, "/admin/object-types", token, map[string]any{
		"code": "supplier", "label": "Supplier", "label_plural": "Suppliers",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create type: status %d, body %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created.ID == 0 {
		t.Fatalf("no type id in response: %s", rr.Body.String())
	}
	typePath := "/admin/object-types/" + strconv.FormatInt(created.ID, 10)

	// 2. Add a "rating" number field capped at 5.
	if rr := do(t, h, http.MethodPost, typePath+"/fields", token, map[string]any{
		"code": "rating", "label": "Rating", "data_type": "number",
		"validation": map[string]any{"max": 5}, "is_required": true,
	}); rr.Code != http.StatusCreated {
		t.Fatalf("create field: status %d, body %s", rr.Code, rr.Body.String())
	}

	// 3. A record that violates the rule is rejected (422).
	bad := do(t, h, http.MethodPost, "/admin/objects/supplier", token, map[string]any{
		"data": map[string]any{"rating": 6},
	})
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid record: status %d, want 422; body %s", bad.Code, bad.Body.String())
	}
	if !strings.Contains(bad.Body.String(), "validation_failed") {
		t.Errorf("expected validation_failed, got %s", bad.Body.String())
	}

	// 4. A valid record is accepted.
	if rr := do(t, h, http.MethodPost, "/admin/objects/supplier", token, map[string]any{
		"data": map[string]any{"rating": 5},
	}); rr.Code != http.StatusCreated {
		t.Fatalf("valid record: status %d, body %s", rr.Code, rr.Body.String())
	}

	// 5. It shows up in the list.
	list := do(t, h, http.MethodGet, "/admin/objects/supplier", token, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list: status %d, body %s", list.Code, list.Body.String())
	}
	var listed struct {
		Total int64 `json:"total"`
		Items []any `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &listed)
	if listed.Total != 1 || len(listed.Items) != 1 {
		t.Errorf("list total=%d items=%d, want 1/1; body %s", listed.Total, len(listed.Items), list.Body.String())
	}

	// 6. The type can't be deleted while it still has records.
	if del := do(t, h, http.MethodDelete, typePath, token, nil); del.Code != http.StatusConflict {
		t.Fatalf("delete type with records: status %d, want 409; body %s", del.Code, del.Body.String())
	}

	// 7. An unknown type code is a 404, not a 500.
	if rr := do(t, h, http.MethodGet, "/admin/objects/nonexistent", token, nil); rr.Code != http.StatusNotFound {
		t.Errorf("unknown type: status %d, want 404", rr.Code)
	}
}
