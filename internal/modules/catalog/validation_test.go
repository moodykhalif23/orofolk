package catalog_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestAttributeValidationEnforcedOnWrite certifies the validation engine is wired
// into product writes end-to-end (real HTTP stack + real Postgres): an attribute
// with a max rule rejects an out-of-range value (422) and accepts an in-range one.
func TestAttributeValidationEnforcedOnWrite(t *testing.T) {
	h, issuer, _ := newServer(t)
	token := catalogToken(t, issuer)

	// Define a number attribute capped at 100.
	if rr := do(t, h, http.MethodPost, "/admin/attributes", token, map[string]any{
		"code": "vtest_weight", "label": "Weight", "data_type": "number",
		"validation": map[string]any{"max": 100},
	}); rr.Code != http.StatusCreated {
		t.Fatalf("create attribute: status %d, body %s", rr.Code, rr.Body.String())
	}

	// A product whose value exceeds the cap is rejected.
	bad := do(t, h, http.MethodPost, "/admin/products", token, map[string]any{
		"sku": "VAL-BAD", "name": "Bad", "slug": "val-bad",
		"attributes": map[string]any{"vtest_weight": 150},
	})
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid product: status %d, want 422; body %s", bad.Code, bad.Body.String())
	}
	if !strings.Contains(bad.Body.String(), "validation_failed") {
		t.Errorf("expected validation_failed in body, got %s", bad.Body.String())
	}

	// An in-range value is accepted.
	ok := do(t, h, http.MethodPost, "/admin/products", token, map[string]any{
		"sku": "VAL-OK", "name": "OK", "slug": "val-ok",
		"attributes": map[string]any{"vtest_weight": 50},
	})
	if ok.Code != http.StatusCreated {
		t.Fatalf("valid product: status %d, want 201; body %s", ok.Code, ok.Body.String())
	}
}
