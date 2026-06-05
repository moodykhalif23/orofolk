package account

import (
	"net/http"
	"time"

	"b2bcommerce/internal/money"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// listBudgets shows the buying company its cost-center budgets with live
// consumption + remaining for the current period — so buyers see their spend
// envelope before it bites at checkout.
func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	p, ok := actor(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no customer context")
		return
	}
	rows, err := h.q.ListBudgetsForCustomer(r.Context(), p.customerID)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list budgets")
		return
	}
	type budget struct {
		CostCenter string `json:"cost_center"`
		Period     string `json:"period"`
		Amount     string `json:"amount"`
		Currency   string `json:"currency"`
		Spent      string `json:"spent"`
		Remaining  string `json:"remaining"`
	}
	now := time.Now()
	out := make([]budget, 0, len(rows))
	for _, b := range rows {
		cc := b.CostCenter
		spent, err := h.q.SpendForCustomerPeriod(r.Context(), gen.SpendForCustomerPeriodParams{
			CustomerID: p.customerID, CostCenter: &cc, CreatedAt: budgetPeriodStart(b.Period, now),
		})
		if err != nil {
			spent = "0"
		}
		remaining, _ := money.Sub(b.Amount, spent)
		out = append(out, budget{
			CostCenter: b.CostCenter, Period: b.Period, Amount: b.Amount, Currency: b.Currency,
			Spent: spent, Remaining: remaining,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// budgetPeriodStart returns the start of the current budget period.
func budgetPeriodStart(period string, now time.Time) time.Time {
	y, m, _ := now.Date()
	loc := now.Location()
	switch period {
	case "annual":
		return time.Date(y, 1, 1, 0, 0, 0, 0, loc)
	case "quarterly":
		qm := time.Month((int(m)-1)/3*3 + 1)
		return time.Date(y, qm, 1, 0, 0, 0, 0, loc)
	default:
		return time.Date(y, m, 1, 0, 0, 0, 0, loc)
	}
}
