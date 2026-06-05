// Package settings implements the hierarchical configuration cascade (PRD §4.3):
// a key can be set at org / website / customer-group / customer scope, and
// resolution returns the most specific value (customer > group > website > org).
// Admin routes manage values; a storefront route resolves a key for the current
// buyer (their website + group + customer), falling back to org defaults.
package settings

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
	h.RoutesWithOptionalAuth(r, authMW, nil)
}

func (h *Handler) RoutesWithOptionalAuth(r chi.Router, authMW, optAuthMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("settings.view")).Get("/admin/settings", h.list)
		ar.With(mw.RequirePermission("settings.manage")).Put("/admin/settings", h.upsert)
		ar.With(mw.RequirePermission("settings.manage")).Delete("/admin/settings/{id}", h.delete)
		ar.With(mw.RequirePermission("settings.view")).Get("/admin/settings/resolve", h.adminResolve)
	})

	r.Group(func(sr chi.Router) {
		if optAuthMW != nil {
			sr.Use(optAuthMW)
		}
		sr.Get("/storefront/settings/{key}", h.storefrontResolve)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

func validScope(s string) bool {
	return s == "org" || s == "website" || s == "group" || s == "customer"
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListConfigSettings(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list settings")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, s := range rows {
		items = append(items, map[string]any{
			"id": s.ID, "scope": s.Scope, "scope_id": s.ScopeID, "key": s.Key, "value": json.RawMessage(s.Value),
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var req struct {
		Scope   string          `json:"scope"`
		ScopeID *int64          `json:"scope_id"`
		Key     string          `json:"key"`
		Value   json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "key is required")
		return
	}
	if !validScope(req.Scope) {
		response.Fail(w, http.StatusBadRequest, "bad_request", "scope must be org, website, group or customer")
		return
	}
	if (req.Scope == "org") != (req.ScopeID == nil) {
		response.Fail(w, http.StatusBadRequest, "bad_request", "scope_id is required for non-org scopes and omitted for org")
		return
	}
	if len(req.Value) == 0 || !json.Valid(req.Value) {
		response.Fail(w, http.StatusBadRequest, "bad_request", "value must be valid JSON")
		return
	}
	s, err := h.q.UpsertConfigSetting(r.Context(), gen.UpsertConfigSettingParams{
		OrganizationID: org, Scope: req.Scope, ScopeID: req.ScopeID, Key: req.Key, Value: req.Value,
	})
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "could not save setting")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"id": s.ID, "scope": s.Scope, "scope_id": s.ScopeID, "key": s.Key, "value": json.RawMessage(s.Value),
	})
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
	n, err := h.q.DeleteConfigSetting(r.Context(), gen.DeleteConfigSettingParams{ID: id, OrganizationID: org})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete setting")
		return
	}
	if n == 0 {
		response.Fail(w, http.StatusNotFound, "not_found", "setting not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolve runs the cascade for a key with the given optional scope ids.
func (h *Handler) resolve(r *http.Request, org int64, key string, website, group, customer *int64) (json.RawMessage, string, bool) {
	row, err := h.q.ResolveConfig(r.Context(), gen.ResolveConfigParams{
		OrganizationID: org, Key: key, Website: website, Grp: group, Customer: customer,
	})
	if err != nil {
		return nil, "", false
	}
	return json.RawMessage(row.Value), row.Scope, true
}

func (h *Handler) adminResolve(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "key is required")
		return
	}
	val, scope, found := h.resolve(r, org, key, qInt(r, "website_id"), qInt(r, "group_id"), qInt(r, "customer_id"))
	if !found {
		response.JSON(w, http.StatusOK, map[string]any{"key": key, "found": false, "value": nil})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"key": key, "found": true, "scope": scope, "value": val})
}

// storefrontResolve resolves a key for the current buyer: their default website,
// their customer group, and their customer id (all optional for anonymous, which
// then only sees org-scoped defaults).
func (h *Handler) storefrontResolve(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	c, _ := mw.ClaimsFrom(r.Context())
	var org int64
	var website, group, customer *int64
	if c != nil {
		org = c.OrgID
		if ws, err := h.q.GetDefaultWebsite(r.Context(), org); err == nil {
			id := ws.ID
			website = &id
		}
		if c.CustomerID != 0 {
			cid := c.CustomerID
			customer = &cid
			if cu, err := h.q.GetCustomer(r.Context(), gen.GetCustomerParams{OrganizationID: org, ID: c.CustomerID}); err == nil {
				group = cu.CustomerGroupID
			}
		}
	}
	if org == 0 {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no organization context")
		return
	}
	val, scope, found := h.resolve(r, org, key, website, group, customer)
	if !found {
		response.JSON(w, http.StatusOK, map[string]any{"key": key, "found": false, "value": nil})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"key": key, "found": true, "scope": scope, "value": val})
}

func qInt(r *http.Request, name string) *int64 {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}
