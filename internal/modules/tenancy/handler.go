// Package tenancy is the admin surface for multi-org / multi-website management
// (PRD §4): list the caller's organization and manage its websites (distinct
// domains, currencies, locales). Storefront host→website resolution lives in the
// catalog module; this module configures the websites it resolves against.
// Admin-only, gated by tenant.view / tenant.manage.
package tenancy

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

type Handler struct {
	q *gen.Queries
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{q: gen.New(pool)} }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("tenant.view")).Get("/admin/organization", h.getOrganization)
		ar.With(mw.RequirePermission("tenant.view")).Get("/admin/websites", h.listWebsites)
		ar.With(mw.RequirePermission("tenant.manage")).Post("/admin/websites", h.createWebsite)
		ar.With(mw.RequirePermission("tenant.view")).Get("/admin/websites/{id}", h.getWebsite)
		ar.With(mw.RequirePermission("tenant.manage")).Put("/admin/websites/{id}", h.updateWebsite)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func (h *Handler) getOrganization(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	o, err := h.q.GetOrganization(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	response.JSON(w, http.StatusOK, o)
}

func (h *Handler) listWebsites(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListWebsites(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list websites")
		return
	}
	if rows == nil {
		rows = []gen.Website{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) getWebsite(w http.ResponseWriter, r *http.Request) {
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
	ws, err := h.q.GetWebsite(r.Context(), gen.GetWebsiteParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "website not found")
		return
	}
	response.JSON(w, http.StatusOK, ws)
}

type websiteInput struct {
	Name            string `json:"name"`
	Domain          string `json:"domain"`
	DefaultCurrency string `json:"default_currency"`
	DefaultLocale   string `json:"default_locale"`
	IsActive        *bool  `json:"is_active"`
}

func (in *websiteInput) valid() bool {
	return in.Name != "" && in.Domain != "" && len(in.DefaultCurrency) == 3
}

func (h *Handler) createWebsite(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var in websiteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || !in.valid() {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name, domain, and a 3-letter currency are required")
		return
	}
	if in.DefaultLocale == "" {
		in.DefaultLocale = "en"
	}
	ws, err := h.q.CreateWebsite(r.Context(), gen.CreateWebsiteParams{
		OrganizationID: org, Name: in.Name, Domain: in.Domain,
		DefaultCurrency: in.DefaultCurrency, DefaultLocale: in.DefaultLocale,
	})
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "could not create website (domain may already be in use)")
		return
	}
	response.JSON(w, http.StatusCreated, ws)
}

func (h *Handler) updateWebsite(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.q.GetWebsite(r.Context(), gen.GetWebsiteParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "website not found")
		return
	}
	var in websiteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || !in.valid() {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name, domain, and a 3-letter currency are required")
		return
	}
	if in.DefaultLocale == "" {
		in.DefaultLocale = existing.DefaultLocale
	}
	active := existing.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	ws, err := h.q.UpdateWebsite(r.Context(), gen.UpdateWebsiteParams{
		OrganizationID: org, ID: id, Name: in.Name, Domain: in.Domain,
		DefaultCurrency: in.DefaultCurrency, DefaultLocale: in.DefaultLocale, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "could not update website (domain may clash)")
		return
	}
	response.JSON(w, http.StatusOK, ws)
}
