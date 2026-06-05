package pricing

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"b2bcommerce/internal/money"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// Admin CRUD for price-adjustment rules (PRD §7.2). The rules are applied at
// price-resolution time in the storefront cart (internal/modules/cart).

func (h *Handler) listAdjustmentRules(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListPriceAdjustmentRules(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list rules")
		return
	}
	if rows == nil {
		rows = []gen.PriceAdjustmentRule{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) createAdjustmentRule(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var req struct {
		Name            string  `json:"name"`
		CustomerGroupID *int64  `json:"customer_group_id"`
		AttributeKey    *string `json:"attribute_key"`
		AttributeValue  *string `json:"attribute_value"`
		AdjustmentType  string  `json:"adjustment_type"`
		AdjustmentValue string  `json:"adjustment_value"`
		Priority        int32   `json:"priority"`
		IsActive        *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.AdjustmentType != "percent" && req.AdjustmentType != "amount" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "adjustment_type must be percent or amount")
		return
	}
	if _, err := money.Parse(req.AdjustmentValue); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "adjustment_value must be numeric")
		return
	}
	// An attribute match needs both key and value, or neither (matches the CHECK).
	if (req.AttributeKey == nil) != (req.AttributeValue == nil) {
		response.Fail(w, http.StatusBadRequest, "bad_request", "attribute_key and attribute_value must be set together")
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	rule, err := h.q.CreatePriceAdjustmentRule(r.Context(), gen.CreatePriceAdjustmentRuleParams{
		OrganizationID: org, Name: req.Name, CustomerGroupID: req.CustomerGroupID,
		AttributeKey: req.AttributeKey, AttributeValue: req.AttributeValue,
		AdjustmentType: req.AdjustmentType, AdjustmentValue: req.AdjustmentValue,
		Priority: req.Priority, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "could not create rule")
		return
	}
	response.JSON(w, http.StatusCreated, rule)
}

func (h *Handler) deleteAdjustmentRule(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	n, err := h.q.DeletePriceAdjustmentRule(r.Context(), gen.DeletePriceAdjustmentRuleParams{ID: id, OrganizationID: org})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete rule")
		return
	}
	if n == 0 {
		response.Fail(w, http.StatusNotFound, "not_found", "rule not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
