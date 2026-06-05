package customers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"b2bcommerce/internal/money"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// Procurement budgets: caps on customer spend per cost-center over a period.
// Enforced at order placement (internal/modules/sales).

func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireCustomer(w, r); !ok {
		return
	}
	id, _ := pathID(r)
	rows, err := h.q.ListBudgetsForCustomer(r.Context(), id)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list budgets")
		return
	}
	if rows == nil {
		rows = []gen.CustomerBudget{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) createBudget(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireCustomer(w, r); !ok {
		return
	}
	id, _ := pathID(r)
	var req struct {
		CostCenter string `json:"cost_center"`
		Period     string `json:"period"`
		Amount     string `json:"amount"`
		Currency   string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if req.Period != "monthly" && req.Period != "quarterly" && req.Period != "annual" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "period must be monthly, quarterly or annual")
		return
	}
	if _, err := money.Parse(req.Amount); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "amount must be numeric")
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	b, err := h.q.CreateBudget(r.Context(), gen.CreateBudgetParams{
		CustomerID: id, CostCenter: req.CostCenter, Period: req.Period, Amount: req.Amount, Currency: req.Currency,
	})
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "could not create budget")
		return
	}
	response.JSON(w, http.StatusCreated, b)
}

func (h *Handler) deleteBudget(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireCustomer(w, r); !ok {
		return
	}
	id, _ := pathID(r)
	bid, err := strconv.ParseInt(chi.URLParam(r, "budgetID"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid budget id")
		return
	}
	n, err := h.q.DeleteBudget(r.Context(), gen.DeleteBudgetParams{ID: bid, CustomerID: id})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete budget")
		return
	}
	if n == 0 {
		response.Fail(w, http.StatusNotFound, "not_found", "budget not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
