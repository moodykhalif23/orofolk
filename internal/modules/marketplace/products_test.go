package marketplace_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// createVendorProduct lists a product through the vendor portal and returns its
// id + slug.
func createVendorProduct(t *testing.T, h http.Handler, tok, sku, name string) (int64, string) {
	t.Helper()
	rr := req(t, h, http.MethodPost, "/vendor/products", tok, map[string]any{
		"sku": sku, "name": name, "status": "active",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create vendor product: want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	var p struct {
		ID             int64  `json:"id"`
		Slug           string `json:"slug"`
		ApprovalStatus string `json:"approval_status"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &p)
	if p.ApprovalStatus != "pending" {
		t.Fatalf("new vendor product approval: want pending, got %q", p.ApprovalStatus)
	}
	return p.ID, p.Slug
}

// TestVendorCatalogModeration proves a vendor-listed product is invisible to the
// storefront until the operator approves it, the moderation queue + approve/reject
// flow, re-submission after rejection, and cross-vendor edit isolation.
func TestVendorCatalogModeration(t *testing.T) {
	h, pool, issuer := newServer(t)
	admin := adminToken(t, issuer, "vendor.view", "vendor.manage")

	vA := mkVendor(t, pool, "Vendor A", "vendor-a", "10")
	tokA := vendorLogin(t, h, pool, vA.ID, "a@vendor.test")
	id, slug := createVendorProduct(t, h, tokA, "VND-1", "Vendor Widget")

	// Pending product is hidden from the public storefront.
	if rr := req(t, h, http.MethodGet, "/storefront/products/"+slug, "", nil); rr.Code != http.StatusNotFound {
		t.Fatalf("pending product on storefront: want 404, got %d (%s)", rr.Code, rr.Body.String())
	}

	// It appears in the operator's moderation queue.
	rr := req(t, h, http.MethodGet, "/admin/products/pending", admin, nil)
	var pending struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &pending)
	if len(pending.Items) != 1 || pending.Items[0].ID != id {
		t.Fatalf("pending queue: want [%d], got %+v", id, pending.Items)
	}

	// Approve -> now visible on the storefront.
	if rr := req(t, h, http.MethodPost, "/admin/products/"+itoa(id)+"/approve", admin, nil); rr.Code != http.StatusOK {
		t.Fatalf("approve: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := req(t, h, http.MethodGet, "/storefront/products/"+slug, "", nil); rr.Code != http.StatusOK {
		t.Fatalf("approved product on storefront: want 200, got %d", rr.Code)
	}
	// Queue is now empty.
	rr = req(t, h, http.MethodGet, "/admin/products/pending", admin, nil)
	_ = json.Unmarshal(rr.Body.Bytes(), &pending)
	if len(pending.Items) != 0 {
		t.Errorf("queue after approve: want empty, got %d", len(pending.Items))
	}

	// Reject flow on a second product, then vendor re-submits.
	id2, slug2 := createVendorProduct(t, h, tokA, "VND-2", "Second Widget")
	if rr := req(t, h, http.MethodPost, "/admin/products/"+itoa(id2)+"/reject", admin, nil); rr.Code != http.StatusOK {
		t.Fatalf("reject: want 200, got %d", rr.Code)
	}
	if rr := req(t, h, http.MethodGet, "/storefront/products/"+slug2, "", nil); rr.Code != http.StatusNotFound {
		t.Errorf("rejected product on storefront: want 404, got %d", rr.Code)
	}
	if rr := req(t, h, http.MethodPost, "/vendor/products/"+itoa(id2)+"/submit", tokA, nil); rr.Code != http.StatusOK {
		t.Errorf("re-submit rejected: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	// Back in the queue.
	rr = req(t, h, http.MethodGet, "/admin/products/pending", admin, nil)
	_ = json.Unmarshal(rr.Body.Bytes(), &pending)
	if len(pending.Items) != 1 || pending.Items[0].ID != id2 {
		t.Errorf("queue after re-submit: want [%d], got %+v", id2, pending.Items)
	}

	// Cross-vendor isolation: vendor B cannot edit vendor A's product.
	vB := mkVendor(t, pool, "Vendor B", "vendor-b", "20")
	tokB := vendorLogin(t, h, pool, vB.ID, "b@vendor.test")
	if rr := req(t, h, http.MethodPut, "/vendor/products/"+itoa(id), tokB, map[string]any{"name": "Hijack"}); rr.Code != http.StatusNotFound {
		t.Errorf("cross-vendor edit: want 404, got %d", rr.Code)
	}

	// And an operator cannot moderate an operator-owned (house) product via the
	// marketplace endpoints (only vendor products are moderatable).
	house := mkProduct(t, pool, "HOUSE-9", "House item", 0)
	if rr := req(t, h, http.MethodPost, "/admin/products/"+itoa(house)+"/approve", admin, nil); rr.Code != http.StatusNotFound {
		t.Errorf("moderate house product: want 404, got %d", rr.Code)
	}
}
