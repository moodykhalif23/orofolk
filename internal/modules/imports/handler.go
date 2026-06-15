// Package imports is the generic import engine (Platform roadmap, Phase 3): an
// upload is parsed, validated and staged as an import_run + per-row import_rows
// (a dry run that touches nothing), previewed, then committed to apply the
// create/update rows. One pipeline serves products and any custom object type,
// each row checked by the same validation engine that guards live writes.
package imports

import (
	"bytes"
	"context"
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

		// Target/template discovery is reachable by an interactive admin
		// (import.view) AND by a scoped supplier key (import.ingest), so a partner
		// can learn a target's schema before feeding it.
		discover := mw.RequireAnyPermission("import.view", "import.ingest")
		ar.With(discover).Get("/admin/imports/targets", h.listTargets)
		ar.With(discover).Get("/admin/imports/template", h.template)

		ar.With(mw.RequirePermission("import.manage")).Post("/admin/imports", h.upload)
		// One-shot partner ingest: validate + apply in a single call (Phase 3
		// slice 4). Scoped to import.ingest — the permission a supplier key carries.
		ar.With(mw.RequirePermission("import.ingest")).Post("/admin/imports/ingest", h.ingest)
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
		items = append(items, map[string]any{
			"key": t.Key(), "label": t.Label(), "columns": t.Columns(), "fields": t.Schema(),
		})
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
	p, done := h.prepare(w, r)
	if done {
		return
	}
	out, create, update, errs := planRows(r.Context(), h.q, p.org, p.target, p.rows)
	run, err := h.stageRun(r.Context(), p.org, p.key, p.format, p.filename, p.opts, out, create, update, errs, actorID(r))
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not stage import")
		return
	}

	audit.SetEntity(r.Context(), "import_runs", run.ID)
	audit.SetSummary(r.Context(), "Staged import of "+p.key+" ("+strconv.Itoa(len(out))+" rows)")

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

// ingest is the one-shot partner path (Phase 3 slice 4): an API-key-scoped
// caller uploads rows that are validated, staged AND applied in a single call —
// no separate dry-run/commit round trip. The run is still recorded so it shows
// in history alongside interactive imports, and every row's outcome is returned
// so a partner can act on rejects. Valid rows apply even when others are
// rejected by validation; an apply that fails at the database rolls the applied
// batch back (all-or-nothing on the applied set, matching commit).
func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	p, done := h.prepare(w, r)
	if done {
		return
	}
	out, create, update, errs := planRows(r.Context(), h.q, p.org, p.target, p.rows)
	run, err := h.stageRun(r.Context(), p.org, p.key, p.format, p.filename, p.opts, out, create, update, errs, actorID(r))
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not stage import")
		return
	}
	applied, fail := h.applyRun(r.Context(), p.org, run, p.target)
	if fail != nil {
		response.Fail(w, fail.status, fail.code, fail.msg)
		return
	}
	if updated, e := h.q.GetImportRun(r.Context(), gen.GetImportRunParams{OrganizationID: p.org, ID: run.ID}); e == nil {
		run = updated // reflect committed status + committed_at
	}

	audit.SetEntity(r.Context(), "import_runs", run.ID)
	audit.SetSummary(r.Context(), "Ingested "+p.key+" ("+strconv.Itoa(applied)+" applied, "+strconv.Itoa(errs)+" rejected)")

	results := make([]map[string]any, 0, len(out))
	for _, p := range out {
		row := map[string]any{"row_number": p.n, "status": p.v.Status}
		if p.v.Message != "" {
			row["message"] = p.v.Message
		}
		results = append(results, row)
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"run": runView(run), "applied": applied, "created": create, "updated": update, "errors": errs,
		"results": results,
	})
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
	applied, fail := h.applyRun(r.Context(), org, run, target)
	if fail != nil {
		response.Fail(w, fail.status, fail.code, fail.msg)
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

// ---- shared pipeline ------------------------------------------------------

// planned is one row's dry-run outcome (1-based row number + verdict).
type planned struct {
	n int
	v Verdict
}

// failure is an HTTP error to surface from a shared step; nil means success.
type failure struct {
	status int
	code   string
	msg    string
}

// prepared is everything the upload and ingest paths share once the request is
// resolved and parsed: the org, the target, its key, the run options and the
// normalized rows ready to plan.
type prepared struct {
	org      int64
	target   Target
	key      string
	opts     importOptions
	rows     []map[string]any
	format   string
	filename string
}

// prepare resolves the target and parses + normalizes the body, shared by the
// dry-run upload and the one-shot ingest. When done=true it has already written
// an error response and the caller must return.
func (h *Handler) prepare(w http.ResponseWriter, r *http.Request) (prepared, bool) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return prepared{}, true
	}
	key := r.URL.Query().Get("target")
	opts := importOptions{
		MatchField: r.URL.Query().Get("match"),
		Normalize:  splitParam(r.URL.Query().Get("normalize")),
	}
	target, err := resolveTarget(r.Context(), h.q, org, key, opts.MatchField)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unknown import target")
		return prepared{}, true
	}
	rows, format, filename, perr := parseRows(r, r.URL.Query().Get("format"))
	if perr != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", perr.Error())
		return prepared{}, true
	}
	normalizeRows(rows, opts.Normalize)
	if len(rows) == 0 {
		response.Fail(w, http.StatusBadRequest, "bad_request", "no rows found")
		return prepared{}, true
	}
	if len(rows) > maxImportRows {
		rows = rows[:maxImportRows]
	}
	return prepared{org: org, target: target, key: key, opts: opts, rows: rows, format: format, filename: filename}, false
}

// parseRows reads the request body into rows per the format. "json" decodes a
// JSON array body; "csv"/"xlsx"/"" read the multipart "file" field (inferring
// the format from the filename when empty). It returns the resolved format and
// the uploaded filename.
func parseRows(r *http.Request, format string) (rows []map[string]any, outFormat, filename string, err error) {
	if format == "json" {
		if e := json.NewDecoder(r.Body).Decode(&rows); e != nil {
			return nil, "", "", errors.New("body must be a JSON array of objects")
		}
		return rows, "json", "", nil
	}
	file, hdr, ferr := r.FormFile("file")
	if ferr != nil {
		return nil, "", "", errors.New("a CSV or XLSX file is required (multipart field 'file')")
	}
	defer file.Close()
	if hdr != nil {
		filename = hdr.Filename
	}
	raw, rerr := io.ReadAll(io.LimitReader(file, 25<<20))
	if rerr != nil {
		return nil, "", "", errors.New("could not read the uploaded file")
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
		return nil, "", "", err
	}
	return rows, format, filename, nil
}

// planRows is the dry run: every row is classified by the target (create /
// update / error) and tallied. Nothing is written.
func planRows(ctx context.Context, q *gen.Queries, org int64, target Target, rows []map[string]any) (out []planned, create, update, errs int) {
	out = make([]planned, 0, len(rows))
	for i, row := range rows {
		v := target.Plan(ctx, q, org, row)
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
	return out, create, update, errs
}

// stageRun persists a dry run — the import_run plus a row per planned outcome —
// in one transaction. Nothing is applied to the target.
func (h *Handler) stageRun(ctx context.Context, org int64, key, format, filename string, opts importOptions, out []planned, create, update, errs int, actor *int64) (gen.ImportRun, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return gen.ImportRun{}, err
	}
	defer tx.Rollback(ctx)
	qtx := h.q.WithTx(tx)
	optsJSON, _ := json.Marshal(opts)
	run, err := qtx.CreateImportRun(ctx, gen.CreateImportRunParams{
		OrganizationID: org, Target: key, Format: format, SourceFilename: filename, Options: optsJSON,
		TotalRows: int32(len(out)), CreateRows: int32(create), UpdateRows: int32(update), ErrorRows: int32(errs),
		CreatedBy: actor,
	})
	if err != nil {
		return gen.ImportRun{}, err
	}
	for _, p := range out {
		data := p.v.Data
		if len(data) == 0 {
			data = []byte("{}")
		}
		if err := qtx.CreateImportRow(ctx, gen.CreateImportRowParams{
			ImportRunID: run.ID, OrganizationID: org, RowNumber: int32(p.n),
			Data: data, Status: p.v.Status, Message: p.v.Message,
		}); err != nil {
			return gen.ImportRun{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return gen.ImportRun{}, err
	}
	return run, nil
}

// applyRun applies a staged run's create/update rows in one transaction and
// marks it committed, returning the number applied. All-or-nothing: a row that
// fails at the database (e.g. a duplicate slug between two rows) rolls the whole
// applied batch back. Shared by interactive commit and one-shot ingest.
func (h *Handler) applyRun(ctx context.Context, org int64, run gen.ImportRun, target Target) (int, *failure) {
	rows, err := h.q.ListCommittableImportRows(ctx, run.ID)
	if err != nil {
		return 0, &failure{http.StatusInternalServerError, "internal", "could not read staged rows"}
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return 0, &failure{http.StatusInternalServerError, "internal", "could not commit import"}
	}
	defer tx.Rollback(ctx)
	qtx := h.q.WithTx(tx)
	applied := 0
	for _, row := range rows {
		if err := target.Apply(ctx, qtx, org, row.Data, row.Status); err != nil {
			return 0, &failure{http.StatusConflict, "conflict", "row " + strconv.Itoa(int(row.RowNumber)) + " could not be applied"}
		}
		applied++
	}
	if err := qtx.MarkImportRunCommitted(ctx, gen.MarkImportRunCommittedParams{OrganizationID: org, ID: run.ID}); err != nil {
		return 0, &failure{http.StatusInternalServerError, "internal", "could not finalize import"}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, &failure{http.StatusInternalServerError, "internal", "could not commit import"}
	}
	return applied, nil
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
