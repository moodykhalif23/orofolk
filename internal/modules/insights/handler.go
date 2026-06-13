// Package insights exposes the executive-analytics engine over HTTP: a live
// metrics view for the dashboard, the persisted weekly digests (AI-narrated
// briefings with anomalies + recommended actions), and an on-demand "generate
// now" trigger. Reading needs report.view; triggering a digest needs
// report.manage (the same permissions as the reporting module).
package insights

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/ai"
	"b2bcommerce/internal/insights"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// Enqueuer schedules an off-request digest generation. *queue.Enqueuer
// satisfies it. When unset, "generate" runs the engine inline on the request.
type Enqueuer interface {
	EnqueueInsightDigest(ctx context.Context, orgID int64, trigger string) error
}

type Handler struct {
	pool     *pgxpool.Pool
	q        *gen.Queries
	narrator ai.Narrator
	enq      Enqueuer
}

func New(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool, q: gen.New(pool), narrator: ai.NewDeterministicNarrator()}
}

// WithNarrator sets the AI narrator used for inline (synchronous) generation.
// Defaults to the deterministic narrator.
func (h *Handler) WithNarrator(n ai.Narrator) *Handler {
	if n != nil {
		h.narrator = n
	}
	return h
}

// WithEnqueuer wires async digest generation; without it "generate" runs inline.
func (h *Handler) WithEnqueuer(e Enqueuer) *Handler { h.enq = e; return h }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		// Static segments are registered before the {id} param route; chi matches
		// literals first, so /latest and /metrics never bind to {id}.
		ar.With(mw.RequirePermission("report.view")).Get("/admin/insights/latest", h.latest)
		ar.With(mw.RequirePermission("report.view")).Get("/admin/insights/metrics", h.metrics)
		ar.With(mw.RequirePermission("report.manage")).Post("/admin/insights/generate", h.generate)
		ar.With(mw.RequirePermission("report.view")).Get("/admin/insights", h.list)
		ar.With(mw.RequirePermission("report.view")).Get("/admin/insights/{id}", h.get)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

// metrics computes the snapshot + anomalies live, on demand — the always-fresh
// view powering the dashboard regardless of when the last digest ran.
func (h *Handler) metrics(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	days := insights.DefaultWindowDays
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 {
		days = d
	}
	snap, err := insights.Build(r.Context(), h.q, org, time.Now(), days)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not compute metrics")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"period_label": snap.PeriodLabel(),
		"period_start": snap.PeriodStart.Format("2006-01-02"),
		"period_end":   snap.PeriodEnd.Format("2006-01-02"),
		"kpis":         snap.KPIs(),
		"anomalies":    insights.Detect(snap),
	})
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	d, err := h.q.LatestInsightDigest(r.Context(), org)
	if errors.Is(err, pgx.ErrNoRows) {
		// No digest yet — omit the field so the client sees an absent (not null)
		// digest and can prompt the operator to generate the first one.
		response.JSON(w, http.StatusOK, map[string]any{})
		return
	}
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not load digest")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"digest": digestResponse(d)})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o > 0 {
		offset = o
	}
	rows, err := h.q.ListInsightDigests(r.Context(), gen.ListInsightDigestsParams{
		OrganizationID: org, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list digests")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		items = append(items, digestResponse(d))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
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
	d, err := h.q.GetInsightDigest(r.Context(), gen.GetInsightDigestParams{ID: id, OrganizationID: org})
	if errors.Is(err, pgx.ErrNoRows) {
		response.Fail(w, http.StatusNotFound, "not_found", "digest not found")
		return
	}
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not load digest")
		return
	}
	response.JSON(w, http.StatusOK, digestResponse(d))
}

// generate triggers a fresh digest. With an enqueuer it runs async (202); inline
// otherwise (200 with the digest) — fine at small scale and in tests.
func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	if h.enq != nil {
		if err := h.enq.EnqueueInsightDigest(r.Context(), org, "manual"); err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not schedule digest")
			return
		}
		response.JSON(w, http.StatusAccepted, map[string]any{"scheduled": true})
		return
	}
	d, err := insights.GenerateDigest(r.Context(), h.q, h.narrator, org, time.Now(), insights.DefaultWindowDays, "manual")
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not generate digest")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"scheduled": false, "digest": digestResponse(d)})
}

// digestResponse renders a stored digest, emitting kpis/anomalies as nested JSON
// (not escaped strings) so the client receives structured objects.
func digestResponse(d gen.InsightDigest) map[string]any {
	return map[string]any{
		"id":           d.ID,
		"period_start": d.PeriodStart.Time.Format("2006-01-02"),
		"period_end":   d.PeriodEnd.Time.Format("2006-01-02"),
		"generated_at": d.GeneratedAt.Format(time.RFC3339),
		"source":       d.Source,
		"trigger":      d.Trigger,
		"narrative":    d.Narrative,
		"kpis":         raw(d.Kpis),
		"anomalies":    raw(d.Anomalies),
	}
}

func raw(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}
