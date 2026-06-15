// Package apikeys is the admin CRUD for programmatic API keys (Platform roadmap,
// Phase 0). The raw secret is returned only at create/rotate; every other view
// shows the non-secret prefix. Scopes are validated against the permissions the
// caller themselves holds, so a key can never exceed its creator's authority.
package apikeys

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/apikey"
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

		ar.With(mw.RequirePermission("apikey.view")).Get("/admin/api-keys", h.list)
		ar.With(mw.RequirePermission("apikey.manage")).Post("/admin/api-keys", h.create)
		ar.With(mw.RequirePermission("apikey.manage")).Post("/admin/api-keys/{id}/rotate", h.rotate)
		ar.With(mw.RequirePermission("apikey.manage")).Delete("/admin/api-keys/{id}", h.revoke)
	})
}

type createInput struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expires_at"` // RFC3339; null = no expiry
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListAPIKeys(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list keys")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, k := range rows {
		items = append(items, keyView(k.ID, k.Name, k.Prefix, k.Scopes, k.LastUsedAt, k.ExpiresAt, k.RevokedAt, k.CreatedAt))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var in createInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	// Least privilege: a key's scopes must be a subset of the creator's own
	// permissions, so a key can never be used to escalate.
	if bad, ok := disallowedScope(r, in.Scopes); !ok {
		response.Fail(w, http.StatusForbidden, "forbidden", "scope not permitted: "+bad)
		return
	}
	expires, err := parseExpiry(in.ExpiresAt)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid expires_at (want RFC3339)")
		return
	}

	raw, prefix, hash, err := apikey.Generate()
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not mint key")
		return
	}
	scopes := in.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	k, err := h.q.CreateAPIKey(r.Context(), gen.CreateAPIKeyParams{
		OrganizationID: org, Name: in.Name, Prefix: prefix, KeyHash: hash,
		Scopes: scopes, ExpiresAt: expires, CreatedBy: actorID(r),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create key")
		return
	}
	view := keyView(k.ID, k.Name, k.Prefix, k.Scopes, k.LastUsedAt, k.ExpiresAt, k.RevokedAt, k.CreatedAt)
	view["key"] = raw // shown exactly once
	response.JSON(w, http.StatusCreated, view)
}

func (h *Handler) rotate(w http.ResponseWriter, r *http.Request) {
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
	raw, prefix, hash, err := apikey.Generate()
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not mint key")
		return
	}
	k, err := h.q.RotateAPIKey(r.Context(), gen.RotateAPIKeyParams{
		OrganizationID: org, ID: id, KeyHash: hash, Prefix: prefix,
	})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "key not found")
		return
	}
	view := keyView(k.ID, k.Name, k.Prefix, k.Scopes, k.LastUsedAt, k.ExpiresAt, k.RevokedAt, k.CreatedAt)
	view["key"] = raw // shown exactly once
	response.JSON(w, http.StatusOK, view)
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
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
	if err := h.q.RevokeAPIKey(r.Context(), gen.RevokeAPIKeyParams{OrganizationID: org, ID: id}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not revoke key")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"revoked": true})
}

// keyView renders a key without ever exposing its hash. status is derived from
// the revoked/expired timestamps. The timestamptz fields are the nullable
// pgtype the typed layer emits.
func keyView(id int64, name, prefix string, scopes []string, lastUsed, expires, revoked pgtype.Timestamptz, createdAt time.Time) map[string]any {
	status := "active"
	switch {
	case revoked.Valid:
		status = "revoked"
	case expires.Valid && expires.Time.Before(time.Now()):
		status = "expired"
	}
	v := map[string]any{
		"id": id, "name": name, "prefix": prefix, "scopes": scopes,
		"status": status, "created_at": createdAt.Format(time.RFC3339),
	}
	if lastUsed.Valid {
		v["last_used_at"] = lastUsed.Time.Format(time.RFC3339)
	}
	if expires.Valid {
		v["expires_at"] = expires.Time.Format(time.RFC3339)
	}
	if revoked.Valid {
		v["revoked_at"] = revoked.Time.Format(time.RFC3339)
	}
	return v
}

func parseExpiry(s *string) (pgtype.Timestamptz, error) {
	if s == nil || *s == "" {
		return pgtype.Timestamptz{}, nil // null = no expiry
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, nil
}

// disallowedScope reports the first requested scope the caller does not hold.
// ok is true when every scope is permitted.
func disallowedScope(r *http.Request, scopes []string) (bad string, ok bool) {
	c, found := mw.ClaimsFrom(r.Context())
	if !found {
		return "", false
	}
	held := make(map[string]struct{}, len(c.Permissions))
	for _, p := range c.Permissions {
		held[p] = struct{}{}
	}
	for _, s := range scopes {
		if _, has := held[s]; !has {
			return s, false
		}
	}
	return "", true
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

func actorID(r *http.Request) *int64 {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return nil
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil {
		return nil
	}
	return &id
}
