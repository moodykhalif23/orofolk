// Package tax (module) exposes admin tax-rate management and a tax-calculation
// preview over the local VAT provider (internal/tax). Order/checkout tax is
// applied in the sales module via the same engine.
package tax

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/money"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
	taxeng "b2bcommerce/internal/tax"
)

type Handler struct {
	q   *gen.Queries
	svc *taxeng.Service
}

func New(pool *pgxpool.Pool) *Handler {
	q := gen.New(pool)
	return &Handler{q: q, svc: taxeng.NewService(q)}
}

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("tax.view")).Get("/admin/tax-rates", h.list)
		ar.With(mw.RequirePermission("tax.manage")).Post("/admin/tax-rates", h.upsert)
		ar.With(mw.RequirePermission("tax.manage")).Delete("/admin/tax-rates/{id}", h.delete)
		ar.With(mw.RequirePermission("tax.view")).Post("/admin/tax/calculate", h.calculate)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListTaxRates(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list rates")
		return
	}
	if rows == nil {
		rows = []gen.TaxRate{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var req struct {
		Country  string `json:"country"`
		TaxClass string `json:"tax_class"`
		Rate     string `json:"rate"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Country) != 2 || req.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "country (2-letter), name required")
		return
	}
	if req.TaxClass == "" {
		req.TaxClass = "standard"
	}
	if req.Rate == "" {
		req.Rate = "0"
	}
	if _, err := money.Parse(req.Rate); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "rate must be a decimal fraction (e.g. 0.16)")
		return
	}
	rate, err := h.q.UpsertTaxRate(r.Context(), gen.UpsertTaxRateParams{
		OrganizationID: org, Country: req.Country, TaxClass: req.TaxClass, Rate: req.Rate, Name: req.Name,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not save rate")
		return
	}
	response.JSON(w, http.StatusCreated, rate)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
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
	if err := h.q.DeleteTaxRate(r.Context(), gen.DeleteTaxRateParams{OrganizationID: org, ID: id}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete rate")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// calculate previews order tax for a destination + product lines, using the
// exact engine order creation uses.
func (h *Handler) calculate(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var req struct {
		Country string `json:"country"`
		Lines   []struct {
			ProductID int64  `json:"product_id"`
			Amount    string `json:"amount"`
		} `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	lines := make([]taxeng.OrderLine, len(req.Lines))
	for i, l := range req.Lines {
		lines[i] = taxeng.OrderLine{ProductID: l.ProductID, Amount: l.Amount}
	}
	perLine, total, err := h.svc.ComputeOrderTax(r.Context(), org, req.Country, lines)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "calculation failed")
		return
	}
	items := make([]map[string]any, len(req.Lines))
	for i, l := range req.Lines {
		items[i] = map[string]any{"product_id": l.ProductID, "amount": l.Amount, "tax_amount": perLine[i]}
	}
	response.JSON(w, http.StatusOK, map[string]any{"country": req.Country, "lines": items, "tax_total": total})
}
