// Package feeds is the syndication module (Platform roadmap, Phase 4): it
// defines feeds — a data source (products or any custom object type) projected
// through an ordered field mapping into a channel format (CSV/JSON/XML) — and
// generates them, on demand or on a schedule. It is the outbound twin of the
// import engine. The generation/source/channel logic lives in internal/feedgen
// (so the background scheduler can drive the same build path); this module owns
// persistence, the HTTP surface, and the signed public delivery URL channels poll.
package feeds

import (
	"errors"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/audit"
	"b2bcommerce/internal/blob"
	"b2bcommerce/internal/feed"
	"b2bcommerce/internal/feedgen"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

const (
	previewRows = 20
	// feedURLTTL is the lifetime of a signed delivery URL. Long, because a channel
	// polls the same URL for months; re-opening the feed mints a fresh one.
	feedURLTTL = 365 * 24 * time.Hour
)

// URLSigner mints/verifies signed capability URLs for the public delivery
// endpoint (satisfied by *auth.Issuer, exactly as the DAM transform URLs are).
type URLSigner interface {
	SignURL(path string, ttl time.Duration) string
	VerifyURL(path, exp, sig string) bool
}

type Handler struct {
	q      *gen.Queries
	svc    *feedgen.Service
	store  blob.Store
	signer URLSigner
}

// New builds the feeds handler. store/signer may be nil — feed CRUD, preview and
// on-the-fly output still work; only the stored artifact + signed delivery URL
// (and thus scheduled feeds) require them.
func New(pool *pgxpool.Pool, store blob.Store, signer URLSigner) *Handler {
	return &Handler{q: gen.New(pool), svc: feedgen.NewService(pool, store), store: store, signer: signer}
}

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Public: a channel polls the signed feed URL (no bearer token), exactly like
	// DAM's /media/file. The signature gates it.
	r.Get("/feeds/{publicID}", h.serve)

	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("feed.view")).Get("/admin/feeds/sources", h.listSources)
		ar.With(mw.RequirePermission("feed.view")).Get("/admin/feeds/channels", h.listChannels)
		ar.With(mw.RequirePermission("feed.view")).Get("/admin/feeds", h.list)
		ar.With(mw.RequirePermission("feed.manage")).Post("/admin/feeds", h.create)
		ar.With(mw.RequirePermission("feed.view")).Get("/admin/feeds/{id}", h.get)
		ar.With(mw.RequirePermission("feed.manage")).Put("/admin/feeds/{id}", h.update)
		ar.With(mw.RequirePermission("feed.manage")).Delete("/admin/feeds/{id}", h.delete)
		ar.With(mw.RequirePermission("feed.view")).Get("/admin/feeds/{id}/preview", h.preview)
		ar.With(mw.RequirePermission("feed.view")).Get("/admin/feeds/{id}/output", h.output)
		ar.With(mw.RequirePermission("feed.manage")).Post("/admin/feeds/{id}/build", h.build)
	})
}

type feedInput struct {
	Name     string       `json:"name"`
	Source   string       `json:"source"`
	Channel  string       `json:"channel"`
	Format   string       `json:"format"`
	Schedule string       `json:"schedule"`
	Mapping  feed.Mapping `json:"mapping"`
	IsActive *bool        `json:"is_active"`
}

func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	sources, err := feedgen.AvailableSources(r.Context(), h.q, org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list sources")
		return
	}
	items := make([]map[string]any, 0, len(sources))
	for _, s := range sources {
		items = append(items, map[string]any{"key": s.Key(), "label": s.Label(), "fields": s.Fields()})
	}
	response.JSON(w, http.StatusOK, map[string]any{"sources": items, "formats": feed.Formats()})
}

// listChannels advertises the destination presets: each channel's expected
// fields, the format it's delivered in, and a starter mapping the builder applies.
func (h *Handler) listChannels(w http.ResponseWriter, _ *http.Request) {
	chans := feedgen.AllChannels()
	items := make([]map[string]any, 0, len(chans))
	for _, c := range chans {
		fields := c.Fields
		if fields == nil {
			fields = []feedgen.ChannelField{}
		}
		preset := c.Preset
		if preset == nil {
			preset = feed.Mapping{}
		}
		items = append(items, map[string]any{
			"id": c.ID, "label": c.Label, "format": c.Format, "fields": fields, "preset": preset,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"channels": items})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListFeeds(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list feeds")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		items = append(items, h.feedView(f))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	in, ch, ok := h.decodeInput(w, r, org)
	if !ok {
		return
	}
	schedule := normalizeSchedule(in.Schedule)
	f, err := h.q.CreateFeed(r.Context(), gen.CreateFeedParams{
		OrganizationID: org, Name: strings.TrimSpace(in.Name), Source: in.Source,
		Channel: ch.ID, Format: ch.EffectiveFormat(in.Format),
		Mapping: mappingJSON(in.Mapping), IsActive: in.IsActive == nil || *in.IsActive,
		Schedule: schedule, NextRunAt: initialNextRun(schedule),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create feed")
		return
	}
	audit.SetEntity(r.Context(), "feeds", f.ID)
	audit.SetSummary(r.Context(), "Created feed "+f.Name)
	response.JSON(w, http.StatusCreated, h.feedView(f))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	in, ch, ok := h.decodeInput(w, r, org)
	if !ok {
		return
	}
	schedule := normalizeSchedule(in.Schedule)
	// Keep the existing next_run_at when the schedule is unchanged so re-saving a
	// feed doesn't reset its cadence; (re)arm it when the schedule changes.
	next := f.NextRunAt
	if schedule != f.Schedule {
		next = initialNextRun(schedule)
	}
	updated, err := h.q.UpdateFeed(r.Context(), gen.UpdateFeedParams{
		OrganizationID: org, ID: f.ID, Name: strings.TrimSpace(in.Name), Source: in.Source,
		Channel: ch.ID, Format: ch.EffectiveFormat(in.Format),
		Mapping: mappingJSON(in.Mapping), IsActive: in.IsActive == nil || *in.IsActive,
		Schedule: schedule, NextRunAt: next,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not update feed")
		return
	}
	audit.SetEntity(r.Context(), "feeds", updated.ID)
	audit.SetSummary(r.Context(), "Updated feed "+updated.Name)
	response.JSON(w, http.StatusOK, h.feedView(updated))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, h.feedView(f))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	if err := h.q.DeleteFeed(r.Context(), gen.DeleteFeedParams{OrganizationID: org, ID: f.ID}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete feed")
		return
	}
	audit.SetEntity(r.Context(), "feeds", f.ID)
	audit.SetSummary(r.Context(), "Deleted feed "+f.Name)
	response.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// preview renders the first rows so an author can see the projection (and the
// channel gap check) without generating the whole document.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	content, n, err := h.svc.Render(r.Context(), org, f, previewRows)
	if err != nil {
		h.failRender(w, err)
		return
	}
	ch, _ := feedgen.ResolveChannel(f.Channel)
	missing := ch.MissingRequired(feed.ParseMapping(f.Mapping))
	if missing == nil {
		missing = []string{}
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"format": ch.EffectiveFormat(f.Format), "channel": ch.ID, "rows": n,
		"content": string(content), "missing_required": missing,
	})
}

// output generates the full feed document fresh and returns it as a download
// (admins always get the current data; channels poll the stored artifact URL).
func (h *Handler) output(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	content, _, err := h.svc.Render(r.Context(), org, f, feedgen.FullLimit)
	if err != nil {
		h.failRender(w, err)
		return
	}
	ch, _ := feedgen.ResolveChannel(f.Channel)
	format := ch.EffectiveFormat(f.Format)
	w.Header().Set("Content-Type", feed.ContentType(format))
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeFilename(f.Name)+"."+feed.Ext(format)+`"`)
	_, _ = w.Write(content)
}

// build regenerates the stored artifact now (the manual path; the scheduler runs
// the same Build for cadenced feeds).
func (h *Handler) build(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	updated, err := h.svc.Build(r.Context(), f)
	if err != nil {
		if errors.Is(err, feedgen.ErrNoStore) {
			response.Fail(w, http.StatusServiceUnavailable, "unavailable", "feed storage is not configured")
			return
		}
		h.failRender(w, err)
		return
	}
	audit.SetEntity(r.Context(), "feeds", updated.ID)
	audit.SetSummary(r.Context(), "Generated feed "+updated.Name)
	response.JSON(w, http.StatusOK, h.feedView(updated))
}

// serve streams the last-built artifact for a feed, gated by the signed URL —
// the public endpoint a channel (Google/Amazon) fetches on its own schedule.
func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	if h.signer == nil || h.store == nil {
		response.Fail(w, http.StatusNotFound, "not_found", "feed delivery not available")
		return
	}
	q := r.URL.Query()
	if !h.signer.VerifyURL(r.URL.Path, q.Get("exp"), q.Get("sig")) {
		response.Fail(w, http.StatusForbidden, "forbidden", "missing or expired signature")
		return
	}
	pid, err := uuid.Parse(chi.URLParam(r, "publicID"))
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "feed not found")
		return
	}
	// Looked up by globally-unique public_id with no org armed (RLS fail-open),
	// the same pattern the inbound webhook/api-key lookups use.
	f, err := h.q.GetFeedByPublicID(r.Context(), pid)
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "feed not found")
		return
	}
	if f.LastArtifactKey == "" {
		response.Fail(w, http.StatusNotFound, "not_found", "feed has not been generated yet")
		return
	}
	rc, err := h.store.Get(r.Context(), f.LastArtifactKey)
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "feed artifact not found")
		return
	}
	defer rc.Close()
	ch, _ := feedgen.ResolveChannel(f.Channel)
	w.Header().Set("Content-Type", feed.ContentType(ch.EffectiveFormat(f.Format)))
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = io.Copy(w, rc)
}

// decodeInput parses the body and validates the source + channel, writing the
// error response itself on failure (ok=false).
func (h *Handler) decodeInput(w http.ResponseWriter, r *http.Request, org int64) (feedInput, feedgen.Channel, bool) {
	var in feedInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Name) == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name is required")
		return in, feedgen.Channel{}, false
	}
	if _, err := feedgen.ResolveSource(r.Context(), h.q, org, in.Source); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown feed source")
		return in, feedgen.Channel{}, false
	}
	ch, ok := feedgen.ResolveChannel(in.Channel)
	if !ok {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown channel")
		return in, feedgen.Channel{}, false
	}
	return in, ch, true
}

func (h *Handler) failRender(w http.ResponseWriter, err error) {
	if errors.Is(err, feedgen.ErrUnknownSource) {
		response.Fail(w, http.StatusConflict, "conflict", "the feed's source no longer exists")
		return
	}
	response.Fail(w, http.StatusInternalServerError, "internal", "could not generate feed")
}

func (h *Handler) loadFeed(w http.ResponseWriter, r *http.Request) (gen.Feed, bool) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return gen.Feed{}, false
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return gen.Feed{}, false
	}
	f, err := h.q.GetFeed(r.Context(), gen.GetFeedParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "feed not found")
		return gen.Feed{}, false
	}
	return f, true
}

// ---- helpers --------------------------------------------------------------

func (h *Handler) feedView(f gen.Feed) map[string]any {
	v := map[string]any{
		"id": f.ID, "public_id": f.PublicID.String(), "name": f.Name,
		"source": f.Source, "channel": f.Channel, "format": f.Format,
		"schedule":   f.Schedule,
		"mapping":    json.RawMessage(orEmptyArray(f.Mapping)),
		"is_active":  f.IsActive,
		"last_bytes": f.LastBytes,
		"created_at": f.CreatedAt.Format(time.RFC3339),
		"updated_at": f.UpdatedAt.Format(time.RFC3339),
	}
	if f.NextRunAt.Valid {
		v["next_run_at"] = f.NextRunAt.Time.Format(time.RFC3339)
	}
	if f.LastBuiltAt.Valid {
		v["last_built_at"] = f.LastBuiltAt.Time.Format(time.RFC3339)
	}
	if f.LastError != "" {
		v["last_error"] = f.LastError
	}
	// A signed, pollable delivery URL (relative path; the channel prepends host).
	if h.signer != nil {
		v["url"] = h.signer.SignURL("/feeds/"+f.PublicID.String(), feedURLTTL)
	}
	return v
}

func normalizeSchedule(s string) string {
	switch s {
	case "hourly", "daily":
		return s
	default:
		return "manual"
	}
}

// initialNextRun arms a scheduled feed to build on the next sweep (now), or
// leaves it null for a manual feed.
func initialNextRun(schedule string) pgtype.Timestamptz {
	if schedule == "manual" {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Now(), Valid: true}
}

func mappingJSON(m feed.Mapping) []byte {
	if len(m) == 0 {
		return []byte("[]")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func orEmptyArray(b []byte) []byte {
	if len(b) == 0 {
		return []byte("[]")
	}
	return b
}

// safeFilename slugs a feed name into a download filename stem.
func safeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ' || r == '-' || r == '_':
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "feed"
	}
	return string(out)
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}
