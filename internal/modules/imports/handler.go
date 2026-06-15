// Package imports is the generic import engine (Platform roadmap, Phase 3): an
// upload is parsed, validated and staged as an import_run + per-row import_rows
// (a dry run that touches nothing), previewed, then committed to apply the
// create/update rows. One pipeline serves products and any custom object type,
// each row checked by the same validation engine that guards live writes.
package imports

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/audit"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

const (
	maxImportRows = 5000
	previewLimit  = 50
)

type Handler struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool, q: gen.New(pool)} }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("import.view")).Get("/admin/imports/targets", h.listTargets)
		ar.With(mw.RequirePermission("import.view")).Get("/admin/imports/template", h.template)
		ar.With(mw.RequirePermission("import.manage")).Post("/admin/imports", h.upload)
		ar.With(mw.RequirePermission("import.view")).Get("/admin/imports/runs", h.listRuns)
		ar.With(mw.RequirePermission("import.view")).Get("/admin/imports/runs/{id}", h.getRun)
		ar.With(mw.RequirePermission("import.view")).Get("/admin/imports/runs/{id}/rows", h.listRows)
		ar.With(mw.RequirePermission("import.manage")).Post("/admin/imports/runs/{id}/commit", h.commit)
	})
}

func (h *Handler) listTargets(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	targets, err := availableTargets(r.Context(), h.q, org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list targets")
		return
	}
	items := make([]map[string]any, 0, len(targets))
	for _, t := range targets {
		items = append(items, map[string]any{"key": t.Key(), "label": t.Label(), "columns": t.Columns()})
	}
	response.JSON(w, http.StatusOK, map[string]any{"targets": items})
}

func (h *Handler) template(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	target, err := resolveTarget(r.Context(), h.q, org, r.URL.Query().Get("target"), "")
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "unknown import target")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	fname := strings.ReplaceAll(target.Key(), ":", "-")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fname+`-template.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write(target.Columns())
	cw.Flush()
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	key := r.URL.Query().Get("target")
	opts := importOptions{
		MatchField: r.URL.Query().Get("match"),
		Normalize:  splitParam(r.URL.Query().Get("normalize")),
	}
	target, err := resolveTarget(r.Context(), h.q, org, key, opts.MatchField)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown import target")
		return
	}
	format := r.URL.Query().Get("format")
	var rows []map[string]any
	var filename string
	if format == "json" {
		if err := json.NewDecoder(r.Body).Decode(&rows); err != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", "body must be a JSON array of objects")
			return
		}
	} else {
		file, hdr, ferr := r.FormFile("file")
		if ferr != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", "a CSV or XLSX file is required (multipart field 'file')")
			return
		}
		defer file.Close()
		if hdr != nil {
			filename = hdr.Filename
		}
		raw, rerr := io.ReadAll(io.LimitReader(file, 25<<20))
		if rerr != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", "could not read the uploaded file")
			return
		}
		if format == "" { // infer from the filename when not given
			if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
				format = "xlsx"
			} else {
				format = "csv"
			}
		}
		if format == "xlsx" {
			rows, err = parseXLSX(raw)
		} else {
			format = "csv"
			rows, err = parseCSV(bytes.NewReader(raw))
		}
		if err != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}
	normalizeRows(rows, opts.Normalize)
	if len(rows) == 0 {
		response.Fail(w, http.StatusBadRequest, "bad_request", "no rows found")
		return
	}
	if len(rows) > maxImportRows {
		rows = rows[:maxImportRows]
	}

	// Dry run: plan every row, tally outcomes — no writes to the target.
	type planned struct {
		n int
		v Verdict
	}
	out := make([]planned, 0, len(rows))
	var create, update, errs int
	for i, row := range rows {
		v := target.Plan(r.Context(), h.q, org, row)
		switch v.Status {
		case "create":
			create++
		case "update":
			update++
		default:
			v.Status = "error"
			errs++
		}
		out = append(out, planned{n: i + 1, v: v})
	}

	// Persist the run and its rows atomically.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not stage import")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.q.WithTx(tx)
	optsJSON, _ := json.Marshal(opts)
	run, err := qtx.CreateImportRun(r.Context(), gen.CreateImportRunParams{
		OrganizationID: org, Target: key, Format: format, SourceFilename: filename, Options: optsJSON,
		TotalRows: int32(len(out)), CreateRows: int32(create), UpdateRows: int32(update), ErrorRows: int32(errs),
		CreatedBy: actorID(r),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create import run")
		return
	}
	for _, p := range out {
		data := p.v.Data
		if len(data) == 0 {
			data = []byte("{}")
		}
		if err := qtx.CreateImportRow(r.Context(), gen.CreateImportRowParams{
			ImportRunID: run.ID, OrganizationID: org, RowNumber: int32(p.n),
			Data: data, Status: p.v.Status, Message: p.v.Message,
		}); err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not stage rows")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not stage import")
		return
	}

	audit.SetEntity(r.Context(), "import_runs", run.ID)
	audit.SetSummary(r.Context(), "Staged import of "+key+" ("+strconv.Itoa(len(out))+" rows)")

	preview := make([]map[string]any, 0, previewLimit)
	for _, p := range out {
		if len(preview) >= previewLimit {
			break
		}
		preview = append(preview, map[string]any{
			"row_number": p.n, "status": p.v.Status, "message": p.v.Message,
			"data": jsonOrEmpty(p.v.Data),
		})
	}
	response.JSON(w, http.StatusCreated, map[string]any{"run": runView(run), "preview": preview})
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListImportRuns(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list runs")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, rn := range rows {
		items = append(items, runView(rn))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, runView(run))
}

func (h *Handler) listRows(w http.ResponseWriter, r *http.Request) {
	run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	limit := atoiDefault(r.URL.Query().Get("page_size"), 100)
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	if page < 1 {
		page = 1
	}
	rows, err := h.q.ListImportRows(r.Context(), gen.ListImportRowsParams{
		ImportRunID: run.ID, Limit: int32(limit), Offset: int32((page - 1) * limit),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list rows")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"row_number": row.RowNumber, "status": row.Status, "message": row.Message,
			"data": jsonOrEmpty(row.Data),
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items, "page": page})
}

func (h *Handler) commit(w http.ResponseWriter, r *http.Request) {
	run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	if run.Status != "validated" {
		response.Fail(w, http.StatusConflict, "conflict", "this import has already been committed")
		return
	}
	org, _ := orgID(r)
	var opts importOptions
	if len(run.Options) > 0 {
		_ = json.Unmarshal(run.Options, &opts)
	}
	target, err := resolveTarget(r.Context(), h.q, org, run.Target, opts.MatchField)
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "the import target no longer exists")
		return
	}
	rows, err := h.q.ListCommittableImportRows(r.Context(), run.ID)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not read staged rows")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not commit import")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.q.WithTx(tx)
	applied := 0
	for _, row := range rows {
		if err := target.Apply(r.Context(), qtx, org, row.Data, row.Status); err != nil {
			// All-or-nothing: the dry run already filtered invalid rows, so a failure
			// here (e.g. a duplicate slug between two rows) rolls the whole batch back.
			response.Fail(w, http.StatusConflict, "conflict", "row "+strconv.Itoa(int(row.RowNumber))+" could not be applied")
			return
		}
		applied++
	}
	if err := qtx.MarkImportRunCommitted(r.Context(), gen.MarkImportRunCommittedParams{OrganizationID: org, ID: run.ID}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not finalize import")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not commit import")
		return
	}

	audit.SetEntity(r.Context(), "import_runs", run.ID)
	audit.SetSummary(r.Context(), "Committed import of "+run.Target+" ("+strconv.Itoa(applied)+" rows)")
	response.JSON(w, http.StatusOK, map[string]any{"committed": applied})
}

func (h *Handler) loadRun(w http.ResponseWriter, r *http.Request) (gen.ImportRun, bool) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return gen.ImportRun{}, false
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return gen.ImportRun{}, false
	}
	run, err := h.q.GetImportRun(r.Context(), gen.GetImportRunParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "import run not found")
		return gen.ImportRun{}, false
	}
	return run, true
}

// ---- helpers -------------------------------------------------------------

func parseCSV(rd io.Reader) ([]map[string]any, error) {
	cr := csv.NewReader(rd)
	cr.FieldsPerRecord = -1 // tolerate ragged rows; map by header
	header, err := cr.Read()
	if err != nil {
		return nil, errors.New("could not read CSV header")
	}
	for i := range header {
		header[i] = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(header[i], "\ufeff")))
	}
	var out []map[string]any
	for {
		rec, e := cr.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, errors.New("malformed CSV row")
		}
		row := make(map[string]any, len(header))
		for i, hname := range header {
			if i < len(rec) {
				row[hname] = rec[i]
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func runView(rn gen.ImportRun) map[string]any {
	v := map[string]any{
		"id": rn.ID, "public_id": rn.PublicID.String(), "target": rn.Target, "format": rn.Format,
		"source_filename": rn.SourceFilename, "status": rn.Status,
		"total_rows": rn.TotalRows, "create_rows": rn.CreateRows, "update_rows": rn.UpdateRows, "error_rows": rn.ErrorRows,
		"created_at": rn.CreatedAt.Format(time.RFC3339),
	}
	if rn.CommittedAt.Valid {
		v["committed_at"] = rn.CommittedAt.Time.Format(time.RFC3339)
	}
	return v
}

func jsonOrEmpty(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
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

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func splitParam(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
