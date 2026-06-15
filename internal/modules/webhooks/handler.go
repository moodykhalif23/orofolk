// Package webhooks is the admin CRUD for outbound webhook endpoints + their
// delivery log (Platform roadmap, Phase 0). The signing secret is returned only
// at create/rotate; other views report has_secret. A failed (or any) delivery
// can be replayed, which re-enqueues the same signed POST.
package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// Replayer re-enqueues a webhook delivery. Satisfied by *queue.Enqueuer.
type Replayer interface {
	EnqueueWebhook(ctx context.Context, org, endpointID int64, event string, payload json.RawMessage) error
}

type Handler struct {
	q   *gen.Queries
	enq Replayer
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{q: gen.New(pool)} }

// WithEnqueuer wires the queue so deliveries can be replayed. Without it, the
// replay endpoint reports it is unavailable.
func (h *Handler) WithEnqueuer(e Replayer) *Handler { h.enq = e; return h }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("webhook.view")).Get("/admin/webhooks", h.list)
		ar.With(mw.RequirePermission("webhook.manage")).Post("/admin/webhooks", h.create)
		ar.With(mw.RequirePermission("webhook.view")).Get("/admin/webhooks/{id}", h.get)
		ar.With(mw.RequirePermission("webhook.manage")).Put("/admin/webhooks/{id}", h.update)
		ar.With(mw.RequirePermission("webhook.manage")).Post("/admin/webhooks/{id}/rotate-secret", h.rotateSecret)
		ar.With(mw.RequirePermission("webhook.manage")).Delete("/admin/webhooks/{id}", h.del)
		ar.With(mw.RequirePermission("webhook.view")).Get("/admin/webhooks/{id}/deliveries", h.deliveries)
		ar.With(mw.RequirePermission("webhook.manage")).Post("/admin/webhooks/deliveries/{id}/replay", h.replay)
	})
}

type endpointInput struct {
	URL         string   `json:"url"`
	Description string   `json:"description"`
	EventTypes  []string `json:"event_types"`
	IsActive    *bool    `json:"is_active"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListWebhookEndpoints(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list webhooks")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, e := range rows {
		items = append(items, endpointView(e))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var in endpointInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.URL == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "url is required")
		return
	}
	secret, err := randomSecret()
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not mint secret")
		return
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	e, err := h.q.CreateWebhookEndpoint(r.Context(), gen.CreateWebhookEndpointParams{
		OrganizationID: org, Url: in.URL, Secret: secret, Description: in.Description,
		EventTypes: nonNil(in.EventTypes), IsActive: active, CreatedBy: actorID(r),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create webhook")
		return
	}
	view := endpointView(e)
	view["secret"] = secret // shown exactly once
	response.JSON(w, http.StatusCreated, view)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	e, ok := h.load(w, r)
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, endpointView(e))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	cur, ok := h.load(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	var in endpointInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.URL == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "url is required")
		return
	}
	active := cur.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	e, err := h.q.UpdateWebhookEndpoint(r.Context(), gen.UpdateWebhookEndpointParams{
		OrganizationID: org, ID: cur.ID, Url: in.URL, Description: in.Description,
		EventTypes: nonNil(in.EventTypes), IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not update webhook")
		return
	}
	response.JSON(w, http.StatusOK, endpointView(e))
}

func (h *Handler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	cur, ok := h.load(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	secret, err := randomSecret()
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not mint secret")
		return
	}
	e, err := h.q.RotateWebhookSecret(r.Context(), gen.RotateWebhookSecretParams{
		OrganizationID: org, ID: cur.ID, Secret: secret,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not rotate secret")
		return
	}
	view := endpointView(e)
	view["secret"] = secret // shown exactly once
	response.JSON(w, http.StatusOK, view)
}

func (h *Handler) del(w http.ResponseWriter, r *http.Request) {
	cur, ok := h.load(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	if err := h.q.DeleteWebhookEndpoint(r.Context(), gen.DeleteWebhookEndpointParams{OrganizationID: org, ID: cur.ID}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete webhook")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *Handler) deliveries(w http.ResponseWriter, r *http.Request) {
	e, ok := h.load(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	rows, err := h.q.ListWebhookDeliveries(r.Context(), gen.ListWebhookDeliveriesParams{OrganizationID: org, EndpointID: e.ID})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list deliveries")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		items = append(items, map[string]any{
			"id": d.ID, "event_type": d.EventType, "status": d.Status,
			"attempt": d.Attempt, "response_status": d.ResponseStatus,
			"error": d.Error, "created_at": d.CreatedAt.Format(time.RFC3339),
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) replay(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	if h.enq == nil {
		response.Fail(w, http.StatusServiceUnavailable, "unavailable", "replay queue not configured")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	d, err := h.q.GetWebhookDelivery(r.Context(), gen.GetWebhookDeliveryParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "delivery not found")
		return
	}
	if err := h.enq.EnqueueWebhook(r.Context(), org, d.EndpointID, d.EventType, json.RawMessage(d.Payload)); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not enqueue replay")
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"replayed": true})
}

func (h *Handler) load(w http.ResponseWriter, r *http.Request) (gen.WebhookEndpoint, bool) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return gen.WebhookEndpoint{}, false
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return gen.WebhookEndpoint{}, false
	}
	e, err := h.q.GetWebhookEndpoint(r.Context(), gen.GetWebhookEndpointParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "webhook not found")
		return gen.WebhookEndpoint{}, false
	}
	return e, true
}

// endpointView renders an endpoint without exposing its signing secret.
func endpointView(e gen.WebhookEndpoint) map[string]any {
	return map[string]any{
		"id": e.ID, "url": e.Url, "description": e.Description,
		"event_types": e.EventTypes, "is_active": e.IsActive,
		"has_secret": e.Secret != "", "created_at": e.CreatedAt.Format(time.RFC3339),
	}
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(b), nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
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
