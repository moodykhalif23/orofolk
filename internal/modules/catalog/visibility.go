package catalog

import (
	"encoding/json"
	"net/http"

	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// hiddenSet resolves the set of product ids hidden from the requesting buyer by
// per-customer/group catalog-visibility rules. Anonymous requests (no storefront
// token) get an empty set — the default catalog is fully visible.
func (h *Handler) hiddenSet(r *http.Request, org int64) (map[int64]bool, error) {
	claims, ok := mw.ClaimsFrom(r.Context())
	if !ok || claims.CustomerID == 0 {
		return map[int64]bool{}, nil
	}
	var cust, grp *int64
	c := claims.CustomerID
	cust = &c
	// A customer's group also carries visibility rules.
	if cu, err := h.q.GetCustomer(r.Context(), gen.GetCustomerParams{OrganizationID: org, ID: claims.CustomerID}); err == nil {
		grp = cu.CustomerGroupID
	}
	ids, err := h.q.HiddenProductIDsForCustomer(r.Context(), gen.HiddenProductIDsForCustomerParams{
		OrganizationID: org, Cust: cust, Grp: grp,
	})
	if err != nil {
		return nil, err
	}
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// ---- admin management (product-level visibility rules) -------------------

func (h *Handler) listVisibility(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	pid, err := pathID(r)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	rows, err := h.q.ListCatalogVisibilityForProduct(r.Context(), gen.ListCatalogVisibilityForProductParams{ProductID: &pid, OrganizationID: org})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list visibility rules")
		return
	}
	if rows == nil {
		rows = []gen.CatalogVisibility{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) createVisibility(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	pid, err := pathID(r)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	// Guard tenant boundary: the product must belong to the caller's org.
	if _, err := h.q.GetProductByID(r.Context(), gen.GetProductByIDParams{OrganizationID: org, ID: pid}); err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "product not found")
		return
	}
	var req struct {
		CustomerID      *int64 `json:"customer_id"`
		CustomerGroupID *int64 `json:"customer_group_id"`
		Visible         *bool  `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	// Exactly one of customer_id / customer_group_id (matches the table CHECK).
	if (req.CustomerID == nil) == (req.CustomerGroupID == nil) {
		response.Fail(w, http.StatusBadRequest, "bad_request", "exactly one of customer_id or customer_group_id is required")
		return
	}
	visible := false // a rule's purpose is almost always to hide
	if req.Visible != nil {
		visible = *req.Visible
	}
	rule, err := h.q.CreateCatalogVisibility(r.Context(), gen.CreateCatalogVisibilityParams{
		ProductID: &pid, CategoryID: nil, CustomerID: req.CustomerID, CustomerGroupID: req.CustomerGroupID, Visible: visible,
	})
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "could not create visibility rule")
		return
	}
	response.JSON(w, http.StatusCreated, rule)
}

func (h *Handler) deleteVisibility(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	id, err := pathID(r)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	n, err := h.q.DeleteCatalogVisibility(r.Context(), gen.DeleteCatalogVisibilityParams{ID: id, OrganizationID: org})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete visibility rule")
		return
	}
	if n == 0 {
		response.Fail(w, http.StatusNotFound, "not_found", "visibility rule not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
