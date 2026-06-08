// Package dam implements Digital Asset Management (Pack 3 §2): media upload with
// checksum dedupe, async preset renditions, signed on-the-fly transforms, and a
// public blob-serving route. It owns all /admin/media + /media/* routes (media
// management was moved here from the CMS module).
package dam

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	_ "image/gif"  // decoder registration for DecodeConfig
	_ "image/jpeg" // "
	_ "image/png"  // "
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "golang.org/x/image/webp" // WebP decoder registration

	"b2bcommerce/internal/blob"
	"b2bcommerce/internal/imageproc"
	"b2bcommerce/internal/queue/jobs"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

const (
	maxUploadBytes = 25 << 20           // per-upload cap for multipart media
	transformTTL   = 7 * 24 * time.Hour // signed transform URL lifetime
)

// RenditionEnqueuer schedules async rendition generation. *queue.Enqueuer
// satisfies it; may be nil (renditions are then generated lazily on first
// transform request instead).
type RenditionEnqueuer interface {
	EnqueueRendition(ctx context.Context, mediaAssetID int64, preset string) error
}

// URLSigner mints/verifies signed capability URLs for the transform endpoint.
// *auth.Issuer satisfies it.
type URLSigner interface {
	SignURL(path string, ttl time.Duration) string
	VerifyURL(path, exp, sig string) bool
}

type Handler struct {
	pool   *pgxpool.Pool
	q      *gen.Queries
	store  blob.Store
	proc   imageproc.Processor
	signer URLSigner
	enq    RenditionEnqueuer
}

func New(pool *pgxpool.Pool, store blob.Store, proc imageproc.Processor, signer URLSigner, enq RenditionEnqueuer) *Handler {
	return &Handler{pool: pool, q: gen.New(pool), store: store, proc: proc, signer: signer, enq: enq}
}

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Public: raw blobs (originals + cached renditions) and signed transforms.
	r.Get("/media/file/*", h.serveFile)
	r.Get("/media/{publicID}/t/{preset}", h.serveTransform)

	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("cms.view")).Get("/admin/media", h.listMedia)
		ar.With(mw.RequirePermission("cms.manage")).Post("/admin/media", h.uploadMedia)
		ar.With(mw.RequirePermission("cms.view")).Get("/admin/media/{id}", h.getMedia)
		ar.With(mw.RequirePermission("cms.manage")).Put("/admin/media/{id}", h.updateMedia)

		ar.With(mw.RequirePermission("cms.view")).Get("/admin/transformation-presets", h.listPresets)
		ar.With(mw.RequirePermission("cms.manage")).Post("/admin/transformation-presets", h.createPreset)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

// uploadMedia stores an uploaded image, dedupes by checksum, and enqueues a
// rendition job per active preset.
func (h *Handler) uploadMedia(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		response.Fail(w, http.StatusRequestEntityTooLarge, "too_large", "upload exceeds limit or is malformed")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "file field required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil || len(data) == 0 {
		response.Fail(w, http.StatusBadRequest, "bad_request", "empty or unreadable file")
		return
	}

	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	// Dedupe: identical bytes reuse the existing asset rather than re-storing.
	if existing, err := h.q.GetMediaByChecksum(r.Context(), gen.GetMediaByChecksumParams{OrganizationID: org, Checksum: &checksum}); err == nil {
		h.renderAsset(w, r, existing, http.StatusOK)
		return
	}

	// Decode dimensions + format (also rejects non-images).
	cfg, format, err := image.DecodeConfig(strings.NewReader(string(data)))
	if err != nil {
		response.Fail(w, http.StatusUnsupportedMediaType, "bad_image", "file is not a supported image")
		return
	}
	ext := extForFormat(format)
	mime := "image/" + format

	key := "org/" + strconv.FormatInt(org, 10) + "/orig/" + uuid.NewString() + "." + ext
	url, err := h.store.Put(r.Context(), key, strings.NewReader(string(data)), mime)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not store file")
		return
	}

	presets, err := h.q.ListPresets(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not load presets")
		return
	}
	status := "ready"
	if len(presets) > 0 && h.enq != nil {
		status = "processing"
	}

	wd, ht := int32(cfg.Width), int32(cfg.Height)
	sz := int64(len(data))
	alt := optForm(r, "alt")
	folder := optForm(r, "folder")
	asset, err := h.q.CreateMediaAsset(r.Context(), gen.CreateMediaAssetParams{
		OrganizationID: org, Url: url, MimeType: &mime, Width: &wd, Height: &ht,
		Alt: alt, Folder: folder, Checksum: &checksum, SizeBytes: &sz, Status: status,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not save media")
		return
	}

	// Tags from a comma-separated "tags" form field.
	for _, tag := range splitTags(r.FormValue("tags")) {
		_ = h.q.AddMediaTag(r.Context(), gen.AddMediaTagParams{MediaAssetID: asset.ID, Tag: tag})
	}
	if status == "processing" {
		for _, p := range presets {
			if err := h.enq.EnqueueRendition(r.Context(), asset.ID, p.Name); err != nil {
				response.Fail(w, http.StatusInternalServerError, "internal", "could not enqueue rendition")
				return
			}
		}
		// Re-read so the response reflects the current status (a synchronous
		// enqueuer will already have flipped it to ready).
		if fresh, err := h.q.GetMediaByID(r.Context(), gen.GetMediaByIDParams{OrganizationID: org, ID: asset.ID}); err == nil {
			asset = fresh
		}
	}
	_ = hdr
	h.renderAsset(w, r, asset, http.StatusCreated)
}

func (h *Handler) listMedia(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListMedia(r.Context(), gen.ListMediaParams{
		Org: org, Folder: optQuery(r, "folder"), Tag: optQuery(r, "tag"), Lim: 200, Off: 0,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list media")
		return
	}
	if rows == nil {
		rows = []gen.MediaAsset{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) getMedia(w http.ResponseWriter, r *http.Request) {
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
	asset, err := h.q.GetMediaByID(r.Context(), gen.GetMediaByIDParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "asset not found")
		return
	}
	h.renderAsset(w, r, asset, http.StatusOK)
}

func (h *Handler) updateMedia(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Alt    *string  `json:"alt"`
		Folder *string  `json:"folder"`
		Tags   []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	asset, err := h.q.UpdateMediaMeta(r.Context(), gen.UpdateMediaMetaParams{
		OrganizationID: org, ID: id, Alt: req.Alt, Folder: req.Folder,
	})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "asset not found")
		return
	}
	if req.Tags != nil {
		_ = h.q.DeleteMediaTags(r.Context(), asset.ID)
		for _, tag := range req.Tags {
			if tag = strings.TrimSpace(tag); tag != "" {
				_ = h.q.AddMediaTag(r.Context(), gen.AddMediaTagParams{MediaAssetID: asset.ID, Tag: tag})
			}
		}
	}
	h.renderAsset(w, r, asset, http.StatusOK)
}

func (h *Handler) listPresets(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListPresets(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list presets")
		return
	}
	if rows == nil {
		rows = []gen.TransformationPreset{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) createPreset(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Width   *int32 `json:"width"`
		Height  *int32 `json:"height"`
		Fit     string `json:"fit"`
		Format  string `json:"format"`
		Quality int32  `json:"quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.Fit == "" {
		req.Fit = "cover"
	}
	if req.Format == "" {
		req.Format = "jpeg"
	}
	if req.Quality <= 0 || req.Quality > 100 {
		req.Quality = 82
	}
	p, err := h.q.CreatePreset(r.Context(), gen.CreatePresetParams{
		OrganizationID: org, Name: req.Name, Width: req.Width, Height: req.Height,
		Fit: req.Fit, Format: req.Format, Quality: req.Quality,
	})
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "preset name may already exist")
		return
	}
	response.JSON(w, http.StatusCreated, p)
}

// serveFile streams a stored blob (original or cached rendition). Public: media
// is generally public on the storefront.
func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "*")
	rc, err := h.store.Get(r.Context(), key)
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "file not found")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", contentTypeForExt(key))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, rc)
}

// serveTransform serves a preset rendition via a signed URL, generating and
// caching it on first request. The signature stops the endpoint being abused to
// generate arbitrary renditions.
func (h *Handler) serveTransform(w http.ResponseWriter, r *http.Request) {
	if h.signer != nil {
		q := r.URL.Query()
		if !h.signer.VerifyURL(r.URL.Path, q.Get("exp"), q.Get("sig")) {
			response.Fail(w, http.StatusForbidden, "forbidden", "missing or expired signature")
			return
		}
	}
	pid, err := uuid.Parse(chi.URLParam(r, "publicID"))
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	preset := chi.URLParam(r, "preset")
	asset, err := h.q.GetMediaByPublicID(r.Context(), pid)
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "asset not found")
		return
	}

	rend, err := h.q.GetRendition(r.Context(), gen.GetRenditionParams{MediaAssetID: asset.ID, Preset: preset})
	if err != nil {
		// Not cached yet — generate synchronously, then re-read.
		if gErr := jobs.GenerateRendition(r.Context(), h.pool, h.store, h.proc, asset.ID, preset); gErr != nil {
			response.Fail(w, http.StatusNotFound, "not_found", "rendition unavailable")
			return
		}
		rend, err = h.q.GetRendition(r.Context(), gen.GetRenditionParams{MediaAssetID: asset.ID, Preset: preset})
		if err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "rendition lookup failed")
			return
		}
	}
	// The rendition URL is a public /media/file path; redirect to it so edge
	// caches key on the stable, unsigned location.
	http.Redirect(w, r, rend.Url, http.StatusFound)
}

// renderAsset returns an asset with its tags, renditions, and signed transform
// URLs for each active preset.
func (h *Handler) renderAsset(w http.ResponseWriter, r *http.Request, a gen.MediaAsset, status int) {
	tags, _ := h.q.ListMediaTags(r.Context(), a.ID)
	if tags == nil {
		tags = []string{}
	}
	rends, _ := h.q.ListRenditions(r.Context(), a.ID)
	if rends == nil {
		rends = []gen.MediaRendition{}
	}
	transforms := map[string]string{}
	if h.signer != nil {
		presets, _ := h.q.ListPresets(r.Context(), a.OrganizationID)
		for _, p := range presets {
			base := "/media/" + a.PublicID.String() + "/t/" + p.Name
			transforms[p.Name] = h.signer.SignURL(base, transformTTL)
		}
	}
	response.JSON(w, status, map[string]any{
		"id": a.ID, "public_id": a.PublicID.String(), "url": a.Url,
		"mime_type": a.MimeType, "width": a.Width, "height": a.Height,
		"alt": a.Alt, "folder": a.Folder, "status": a.Status,
		"checksum": a.Checksum, "size_bytes": a.SizeBytes,
		"tags": tags, "renditions": rends, "transforms": transforms,
	})
}

// ---- helpers --------------------------------------------------------------

func extForFormat(format string) string {
	switch format {
	case "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	case "webp":
		return "webp"
	default:
		return "bin"
	}
}

func contentTypeForExt(key string) string {
	switch strings.ToLower(strings.TrimPrefix(path.Ext(key), ".")) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func splitTags(csv string) []string {
	var out []string
	for _, t := range strings.Split(csv, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func optForm(r *http.Request, key string) *string {
	if v := strings.TrimSpace(r.FormValue(key)); v != "" {
		return &v
	}
	return nil
}

func optQuery(r *http.Request, key string) *string {
	if v := strings.TrimSpace(r.URL.Query().Get(key)); v != "" {
		return &v
	}
	return nil
}
