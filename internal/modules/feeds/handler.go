// Package feeds is the syndication module (Platform roadmap, Phase 4): it
// defines feeds — a data source (products or any custom object type) projected
// through an ordered field mapping into a channel format (CSV/JSON/XML) — and
// generates them on demand. It is the outbound twin of the import engine: the
// same products/object duo, the same map-the-fields shape, run in reverse. The
// pure rendering lives in internal/feed; this module owns persistence, source
// resolution and the HTTP surface. Scheduled regeneration + delivery, channel
// presets, and the builder UI land in later slices.
package feeds

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/audit"
	"b2bcommerce/internal/feed"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

const previewRows = 20

type Handler struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool, q: gen.New(pool)} }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
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
	})
}

type feedInput struct {
	Name     string       `json:"name"`
	Source   string       `json:"source"`
	Channel  string       `json:"channel"`
	Format   string       `json:"format"`
	Mapping  feed.Mapping `json:"mapping"`
	IsActive *bool        `json:"is_active"`
}

func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	sources, err := availableSources(r.Context(), h.q, org)
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
// fields, the format it's delivered in, and a starter mapping the builder can
// apply. No org data — a static registry.
func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request) {
	chans := allChannels()
	items := make([]map[string]any, 0, len(chans))
	for _, c := range chans {
		fields := c.fields
		if fields == nil {
			fields = []channelField{}
		}
		preset := c.preset
		if preset == nil {
			preset = feed.Mapping{}
		}
		items = append(items, map[string]any{
			"id": c.id, "label": c.label, "format": c.format, "fields": fields, "preset": preset,
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
		items = append(items, feedView(f))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var in feedInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Name) == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if _, err := resolveSource(r.Context(), h.q, org, in.Source); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown feed source")
		return
	}
	ch, ok := resolveChannel(in.Channel)
	if !ok {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown channel")
		return
	}
	f, err := h.q.CreateFeed(r.Context(), gen.CreateFeedParams{
		OrganizationID: org, Name: strings.TrimSpace(in.Name), Source: in.Source,
		Channel: ch.id, Format: ch.effectiveFormat(in.Format),
		Mapping: mappingJSON(in.Mapping), IsActive: in.IsActive == nil || *in.IsActive,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create feed")
		return
	}
	audit.SetEntity(r.Context(), "feeds", f.ID)
	audit.SetSummary(r.Context(), "Created feed "+f.Name)
	response.JSON(w, http.StatusCreated, feedView(f))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, feedView(f))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	var in feedInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Name) == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if _, err := resolveSource(r.Context(), h.q, org, in.Source); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown feed source")
		return
	}
	ch, ok := resolveChannel(in.Channel)
	if !ok {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown channel")
		return
	}
	updated, err := h.q.UpdateFeed(r.Context(), gen.UpdateFeedParams{
		OrganizationID: org, ID: f.ID, Name: strings.TrimSpace(in.Name), Source: in.Source,
		Channel: ch.id, Format: ch.effectiveFormat(in.Format),
		Mapping: mappingJSON(in.Mapping), IsActive: in.IsActive == nil || *in.IsActive,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not update feed")
		return
	}
	audit.SetEntity(r.Context(), "feeds", updated.ID)
	audit.SetSummary(r.Context(), "Updated feed "+updated.Name)
	response.JSON(w, http.StatusOK, feedView(updated))
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

// preview renders the first rows so an author can see the projection without
// generating the whole document.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	content, n, fail := h.render(r, f, previewRows)
	if fail != nil {
		response.Fail(w, fail.status, fail.code, fail.msg)
		return
	}
	ch, _ := resolveChannel(f.Channel)
	missing := ch.missingRequired(feed.ParseMapping(f.Mapping))
	if missing == nil {
		missing = []string{}
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"format": ch.effectiveFormat(f.Format), "channel": ch.id, "rows": n,
		"content": string(content), "missing_required": missing,
	})
}

// output generates the full feed document and returns it as a download.
func (h *Handler) output(w http.ResponseWriter, r *http.Request) {
	f, ok := h.loadFeed(w, r)
	if !ok {
		return
	}
	content, _, fail := h.render(r, f, feedRowCap)
	if fail != nil {
		response.Fail(w, fail.status, fail.code, fail.msg)
		return
	}
	w.Header().Set("Content-Type", feed.ContentType(f.Format))
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeFilename(f.Name)+"."+feed.Ext(f.Format)+`"`)
	_, _ = w.Write(content)
}

type failure struct {
	status int
	code   string
	msg    string
}

// render resolves the feed's source, pulls up to limit rows and projects them
// through the mapping into the feed's format.
func (h *Handler) render(r *http.Request, f gen.Feed, limit int) ([]byte, int, *failure) {
	org, _ := orgID(r)
	src, err := resolveSource(r.Context(), h.q, org, f.Source)
	if err != nil {
		return nil, 0, &failure{http.StatusConflict, "conflict", "the feed's source no longer exists"}
	}
	rows, err := src.Rows(r.Context(), h.q, org, limit)
	if err != nil {
		return nil, 0, &failure{http.StatusInternalServerError, "internal", "could not read source"}
	}
	ch, _ := resolveChannel(f.Channel)
	content, err := feed.RenderWith(ch.effectiveFormat(f.Format), feed.ParseMapping(f.Mapping), rows, feed.RenderOpts{XML: ch.envelope})
	if err != nil {
		return nil, 0, &failure{http.StatusInternalServerError, "internal", "could not render feed"}
	}
	return content, len(rows), nil
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

func feedView(f gen.Feed) map[string]any {
	return map[string]any{
		"id": f.ID, "public_id": f.PublicID.String(), "name": f.Name,
		"source": f.Source, "channel": f.Channel, "format": f.Format,
		"mapping":    json.RawMessage(orEmptyArray(f.Mapping)),
		"is_active":  f.IsActive,
		"created_at": f.CreatedAt.Format(time.RFC3339),
		"updated_at": f.UpdatedAt.Format(time.RFC3339),
	}
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
