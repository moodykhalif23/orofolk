// Package dataquality surfaces catalog data-health (Platform roadmap, Phase 1):
// how complete the catalog is against the REQUIRED attributes each product's
// attribute family declares. It is the first answer to "is my catalog complete
// and correct?" — an org-level score plus the worst offenders and exactly which
// required attributes each is missing (the enrichment work-list).
package dataquality

import (
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

const (
	defaultWorstLimit = 20
	maxWorstLimit     = 200
)

type Handler struct {
	q *gen.Queries
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{q: gen.New(pool)} }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))
		ar.With(mw.RequirePermission("dataquality.view")).Get("/admin/data-health/catalog", h.catalog)
	})
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	limit := defaultWorstLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxWorstLimit {
		limit = maxWorstLimit
	}

	sum, err := h.q.CatalogCompletenessSummary(r.Context(), c.OrgID)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not compute completeness")
		return
	}
	worst, err := h.q.CatalogCompletenessWorst(r.Context(), gen.CatalogCompletenessWorstParams{
		OrganizationID: c.OrgID, RowLimit: int32(limit),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list incomplete products")
		return
	}

	items := make([]map[string]any, 0, len(worst))
	for _, p := range worst {
		pct := 0.0
		if p.RequiredTotal > 0 {
			pct = float64(p.RequiredPresent) / float64(p.RequiredTotal) * 100
		}
		items = append(items, map[string]any{
			"id": p.ID, "sku": p.Sku, "name": p.Name,
			"required_total": p.RequiredTotal, "required_present": p.RequiredPresent,
			"completeness": round1(pct), "missing": p.Missing,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"products_total":       sum.ProductsTotal,
			"products_with_family": sum.ProductsWithFamily,
			"products_scored":      sum.ProductsScored,
			"avg_completeness":     round1(sum.AvgCompleteness),
			"complete_count":       sum.CompleteCount,
			"incomplete_count":     sum.IncompleteCount,
		},
		"worst": items,
	})
}

// round1 rounds a percentage to one decimal place for display.
func round1(f float64) float64 { return math.Round(f*10) / 10 }
