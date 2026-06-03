// Package erp implements ERP/accounting sync (Pack 2 §4.6): admin-managed
// integration connections, an idempotent outbound sweep (confirmed orders +
// issued invoices → ERP, recorded in external_refs/sync_logs), and a signed
// inbound webhook for master data (inventory). The transport is the generic
// signed-webhook connector in internal/erp.
package erp

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	erpconn "b2bcommerce/internal/erp"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

const sweepBatch = 100

type Handler struct {
	pool      *pgxpool.Pool
	q         *gen.Queries
	connector *erpconn.Connector
}

func New(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool, q: gen.New(pool), connector: erpconn.NewConnector()}
}

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Inbound ERP webhook (HMAC-verified, not bearer-gated).
	r.Post("/webhooks/erp/{connectionID}", h.inbound)

	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("erp.view")).Get("/admin/erp/connections", h.listConnections)
		ar.With(mw.RequirePermission("erp.manage")).Post("/admin/erp/connections", h.createConnection)
		ar.With(mw.RequirePermission("erp.view")).Get("/admin/erp/connections/{id}", h.getConnection)
		ar.With(mw.RequirePermission("erp.manage")).Put("/admin/erp/connections/{id}", h.updateConnection)
		ar.With(mw.RequirePermission("erp.manage")).Post("/admin/erp/connections/{id}/sync", h.runSync)
		ar.With(mw.RequirePermission("erp.view")).Get("/admin/erp/sync-logs", h.listSyncLogs)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

// renderConnection omits the secret (returns has_secret instead).
func renderConnection(c gen.IntegrationConnection) map[string]any {
	return map[string]any{
		"id": c.ID, "provider": c.Provider, "kind": c.Kind, "endpoint": c.Endpoint,
		"has_secret": c.Secret != nil && *c.Secret != "", "config": json.RawMessage(nonEmpty(c.Config)),
		"is_active": c.IsActive, "created_at": c.CreatedAt.Format(time.RFC3339),
	}
}

func nonEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

type connInput struct {
	Provider string          `json:"provider"`
	Kind     string          `json:"kind"`
	Endpoint *string         `json:"endpoint"`
	Secret   *string         `json:"secret"`
	Config   json.RawMessage `json:"config"`
	IsActive *bool           `json:"is_active"`
}

func (h *Handler) listConnections(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListIntegrationConnections(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list connections")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		items = append(items, renderConnection(c))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createConnection(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var in connInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Provider == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "provider is required")
		return
	}
	if in.Kind != "accounting" {
		in.Kind = "erp"
	}
	cfg := []byte("{}")
	if len(in.Config) > 0 {
		cfg = in.Config
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	c, err := h.q.CreateIntegrationConnection(r.Context(), gen.CreateIntegrationConnectionParams{
		OrganizationID: org, Provider: in.Provider, Kind: in.Kind, Endpoint: in.Endpoint,
		Secret: in.Secret, Config: cfg, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create connection")
		return
	}
	response.JSON(w, http.StatusCreated, renderConnection(c))
}

func (h *Handler) getConnection(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadConnection(w, r)
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, renderConnection(c))
}

func (h *Handler) updateConnection(w http.ResponseWriter, r *http.Request) {
	cur, ok := h.loadConnection(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	var in connInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Provider == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "provider is required")
		return
	}
	if in.Kind != "accounting" {
		in.Kind = "erp"
	}
	secret := cur.Secret
	if in.Secret != nil {
		secret = in.Secret
	}
	cfg := cur.Config
	if len(in.Config) > 0 {
		cfg = in.Config
	}
	active := cur.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	c, err := h.q.UpdateIntegrationConnection(r.Context(), gen.UpdateIntegrationConnectionParams{
		OrganizationID: org, ID: cur.ID, Provider: in.Provider, Kind: in.Kind,
		Endpoint: in.Endpoint, Secret: secret, Config: cfg, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not update connection")
		return
	}
	response.JSON(w, http.StatusOK, renderConnection(c))
}

func (h *Handler) loadConnection(w http.ResponseWriter, r *http.Request) (gen.IntegrationConnection, bool) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return gen.IntegrationConnection{}, false
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return gen.IntegrationConnection{}, false
	}
	c, err := h.q.GetIntegrationConnection(r.Context(), gen.GetIntegrationConnectionParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "connection not found")
		return gen.IntegrationConnection{}, false
	}
	return c, true
}

func (h *Handler) listSyncLogs(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListSyncLogs(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list logs")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, l := range rows {
		items = append(items, map[string]any{
			"id": l.ID, "connection_id": l.ConnectionID, "direction": l.Direction,
			"entity_type": l.EntityType, "entity_id": l.EntityID, "operation": l.Operation,
			"status": l.Status, "external_id": l.ExternalID, "error": l.Error,
			"created_at": l.CreatedAt.Format(time.RFC3339),
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) runSync(w http.ResponseWriter, r *http.Request) {
	c, ok := h.loadConnection(w, r)
	if !ok {
		return
	}
	synced, errs := h.Sweep(r.Context(), c)
	response.JSON(w, http.StatusOK, map[string]any{"synced": synced, "errors": errs})
}

// SweepAll runs the outbound sweep for every active connection (all orgs). Used
// by the periodic river job.
func (h *Handler) SweepAll(ctx context.Context) (int, error) {
	conns, err := h.q.ListActiveIntegrationConnections(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, c := range conns {
		n, _ := h.Sweep(ctx, c)
		total += n
	}
	return total, nil
}

// Sweep pushes every not-yet-synced confirmed order and issued invoice to the
// connection's ERP endpoint. Idempotent: already-synced entities (present in
// external_refs) are skipped, so re-running is safe. Returns counts.
func (h *Handler) Sweep(ctx context.Context, c gen.IntegrationConnection) (synced, errs int) {
	if !c.IsActive {
		return 0, 0
	}
	org := c.OrganizationID

	orders, _ := h.q.ListOrdersToSync(ctx, gen.ListOrdersToSyncParams{OrganizationID: org, ConnectionID: c.ID, Limit: sweepBatch})
	for _, o := range orders {
		payload := map[string]any{
			"public_id": o.PublicID.String(), "entity": "order",
			"order_number": "ORD-" + o.PublicID.String()[:8], "customer_id": o.CustomerID,
			"currency": o.Currency, "subtotal": o.Subtotal, "tax_total": o.TaxTotal,
			"grand_total": o.GrandTotal, "status": o.Status, "po_number": o.PoNumber,
		}
		if h.pushEntity(ctx, c, "order", o.ID, o.PublicID.String(), payload) {
			synced++
		} else {
			errs++
		}
	}

	invoices, _ := h.q.ListInvoicesToSync(ctx, gen.ListInvoicesToSyncParams{OrganizationID: org, ConnectionID: c.ID, Limit: sweepBatch})
	for _, inv := range invoices {
		payload := map[string]any{
			"public_id": inv.PublicID.String(), "entity": "invoice",
			"order_id": inv.OrderID, "currency": inv.Currency, "status": inv.Status,
			"subtotal": inv.Subtotal, "tax_total": inv.TaxTotal, "grand_total": inv.GrandTotal,
		}
		if h.pushEntity(ctx, c, "invoice", inv.ID, inv.PublicID.String(), payload) {
			synced++
		} else {
			errs++
		}
	}
	return synced, errs
}

// pushEntity sends one document and records external_refs + sync_logs.
func (h *Handler) pushEntity(ctx context.Context, c gen.IntegrationConnection, entityType string, entityID int64, publicID string, payload map[string]any) bool {
	body, _ := json.Marshal(payload)
	key := erpconn.IdempotencyKey(entityType, entityID, "upsert")
	secret := ""
	if c.Secret != nil {
		secret = *c.Secret
	}
	res, err := h.connector.Push(ctx, deref(c.Endpoint), secret, key, body)
	if err != nil {
		msg := err.Error()
		_, _ = h.q.CreateSyncLog(ctx, gen.CreateSyncLogParams{
			OrganizationID: c.OrganizationID, ConnectionID: c.ID, Direction: "outbound",
			EntityType: entityType, EntityID: &entityID, Operation: "upsert", Status: "error",
			IdempotencyKey: &key, Error: &msg,
		})
		return false
	}
	extID := res.ExternalID
	if extID == "" {
		extID = publicID
	}
	_, _ = h.q.CreateExternalRef(ctx, gen.CreateExternalRefParams{
		ConnectionID: c.ID, EntityType: entityType, EntityID: entityID, ExternalID: extID,
	})
	_, _ = h.q.CreateSyncLog(ctx, gen.CreateSyncLogParams{
		OrganizationID: c.OrganizationID, ConnectionID: c.ID, Direction: "outbound",
		EntityType: entityType, EntityID: &entityID, Operation: "upsert", Status: "sent",
		IdempotencyKey: &key, ExternalID: &extID,
	})
	return true
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
