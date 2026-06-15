// Package objects implements generic data modeling (Platform roadmap, Phase 2):
// org-defined custom object TYPES with a FIELD schema, and CRUD over RECORDS of
// each type. Records are stored as JSONB and validated against their type's
// fields by the same engine that guards product attributes (internal/validation),
// so the modeling layer generalizes products to any entity without new code per
// type. Everything is org-scoped (RLS), audited (the staff-mutation middleware),
// and permission-gated (object.view / object.manage).
package objects

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/audit"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/validation"
)

var allowedDataTypes = map[string]bool{
	"text": true, "number": true, "boolean": true, "select": true,
	"multiselect": true, "date": true, "file": true, "price": true,
}

type Handler struct {
	q *gen.Queries
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{q: gen.New(pool)} }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		// Modeling: object types + their field schema.
		ar.With(mw.RequirePermission("object.view")).Get("/admin/object-types", h.listTypes)
		ar.With(mw.RequirePermission("object.manage")).Post("/admin/object-types", h.createType)
		ar.With(mw.RequirePermission("object.view")).Get("/admin/object-types/{id}", h.getType)
		ar.With(mw.RequirePermission("object.manage")).Put("/admin/object-types/{id}", h.updateType)
		ar.With(mw.RequirePermission("object.manage")).Delete("/admin/object-types/{id}", h.deleteType)
		ar.With(mw.RequirePermission("object.manage")).Post("/admin/object-types/{id}/fields", h.createField)
		ar.With(mw.RequirePermission("object.manage")).Put("/admin/object-fields/{id}", h.updateField)
		ar.With(mw.RequirePermission("object.manage")).Delete("/admin/object-fields/{id}", h.deleteField)

		// Records, addressed by their type's code.
		ar.With(mw.RequirePermission("object.view")).Get("/admin/objects/{code}", h.listRecords)
		ar.With(mw.RequirePermission("object.manage")).Post("/admin/objects/{code}", h.createRecord)
		ar.With(mw.RequirePermission("object.view")).Get("/admin/objects/{code}/{id}", h.getRecord)
		ar.With(mw.RequirePermission("object.manage")).Put("/admin/objects/{code}/{id}", h.updateRecord)
		ar.With(mw.RequirePermission("object.manage")).Delete("/admin/objects/{code}/{id}", h.deleteRecord)
	})
}

// ---- Object types --------------------------------------------------------

func (h *Handler) listTypes(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListObjectTypes(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list object types")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		items = append(items, typeView(t))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type typeInput struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	LabelPlural string `json:"label_plural"`
	Description string `json:"description"`
	IsActive    *bool  `json:"is_active"`
}

func (h *Handler) createType(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var in typeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Code == "" || in.Label == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "code and label are required")
		return
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	t, err := h.q.CreateObjectType(r.Context(), gen.CreateObjectTypeParams{
		OrganizationID: org, Code: in.Code, Label: in.Label,
		LabelPlural: in.LabelPlural, Description: in.Description, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "could not create object type (duplicate code?)")
		return
	}
	audit.SetEntity(r.Context(), "object_types", t.ID)
	audit.SetSummary(r.Context(), "Created object type "+t.Code)
	response.JSON(w, http.StatusCreated, typeView(t))
}

func (h *Handler) getType(w http.ResponseWriter, r *http.Request) {
	t, ok := h.loadType(w, r)
	if !ok {
		return
	}
	fields, err := h.q.ListObjectFieldsForType(r.Context(), t.ID)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not load fields")
		return
	}
	fv := make([]map[string]any, 0, len(fields))
	for _, f := range fields {
		fv = append(fv, fieldView(f))
	}
	out := typeView(t)
	out["fields"] = fv
	response.JSON(w, http.StatusOK, out)
}

func (h *Handler) updateType(w http.ResponseWriter, r *http.Request) {
	t, ok := h.loadType(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	var in typeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Label == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "label is required")
		return
	}
	active := t.IsActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	upd, err := h.q.UpdateObjectType(r.Context(), gen.UpdateObjectTypeParams{
		OrganizationID: org, ID: t.ID, Label: in.Label,
		LabelPlural: in.LabelPlural, Description: in.Description, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not update object type")
		return
	}
	audit.SetEntity(r.Context(), "object_types", upd.ID)
	audit.SetSummary(r.Context(), "Updated object type "+upd.Code)
	response.JSON(w, http.StatusOK, typeView(upd))
}

func (h *Handler) deleteType(w http.ResponseWriter, r *http.Request) {
	t, ok := h.loadType(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	// A type with records can't be dropped — that would orphan data. Disable it
	// (is_active=false) or delete its records first.
	n, err := h.q.CountObjectRecordsForType(r.Context(), t.ID)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not check records")
		return
	}
	if n > 0 {
		response.Fail(w, http.StatusConflict, "conflict", "this type still has records — delete them or disable the type instead")
		return
	}
	if err := h.q.DeleteObjectType(r.Context(), gen.DeleteObjectTypeParams{OrganizationID: org, ID: t.ID}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete object type")
		return
	}
	audit.SetEntity(r.Context(), "object_types", t.ID)
	audit.SetSummary(r.Context(), "Deleted object type "+t.Code)
	response.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *Handler) loadType(w http.ResponseWriter, r *http.Request) (gen.ObjectType, bool) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return gen.ObjectType{}, false
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return gen.ObjectType{}, false
	}
	t, err := h.q.GetObjectType(r.Context(), gen.GetObjectTypeParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "object type not found")
		return gen.ObjectType{}, false
	}
	return t, true
}

// ---- Object fields -------------------------------------------------------

type fieldInput struct {
	Code       string          `json:"code"`
	Label      string          `json:"label"`
	DataType   string          `json:"data_type"`
	Options    json.RawMessage `json:"options"`
	Validation json.RawMessage `json:"validation"`
	IsRequired bool            `json:"is_required"`
	SortOrder  int32           `json:"sort_order"`
}

func (h *Handler) createField(w http.ResponseWriter, r *http.Request) {
	t, ok := h.loadType(w, r) // {id} is the type id here
	if !ok {
		return
	}
	org, _ := orgID(r)
	var in fieldInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Code == "" || in.Label == "" || in.DataType == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "code, label and data_type are required")
		return
	}
	if !allowedDataTypes[in.DataType] {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unsupported data_type")
		return
	}
	f, err := h.q.CreateObjectField(r.Context(), gen.CreateObjectFieldParams{
		ObjectTypeID: t.ID, OrganizationID: org, Code: in.Code, Label: in.Label, DataType: in.DataType,
		Options: in.Options, Validation: jsonOrEmpty(in.Validation), IsRequired: in.IsRequired, SortOrder: in.SortOrder,
	})
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "could not create field (duplicate code?)")
		return
	}
	audit.SetEntity(r.Context(), "object_fields", f.ID)
	audit.SetSummary(r.Context(), "Added field "+f.Code+" to "+t.Code)
	response.JSON(w, http.StatusCreated, fieldView(f))
}

func (h *Handler) updateField(w http.ResponseWriter, r *http.Request) {
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
	var in fieldInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Label == "" || in.DataType == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "label and data_type are required")
		return
	}
	if !allowedDataTypes[in.DataType] {
		response.Fail(w, http.StatusBadRequest, "bad_request", "unsupported data_type")
		return
	}
	f, err := h.q.UpdateObjectField(r.Context(), gen.UpdateObjectFieldParams{
		OrganizationID: org, ID: id, Label: in.Label, DataType: in.DataType,
		Options: in.Options, Validation: jsonOrEmpty(in.Validation), IsRequired: in.IsRequired, SortOrder: in.SortOrder,
	})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "field not found")
		return
	}
	response.JSON(w, http.StatusOK, fieldView(f))
}

func (h *Handler) deleteField(w http.ResponseWriter, r *http.Request) {
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
	if err := h.q.DeleteObjectField(r.Context(), gen.DeleteObjectFieldParams{OrganizationID: org, ID: id}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete field")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ---- Object records ------------------------------------------------------

func (h *Handler) listRecords(w http.ResponseWriter, r *http.Request) {
	t, ok := h.resolveType(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	limit := atoiDefault(r.URL.Query().Get("page_size"), 25)
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	rows, err := h.q.ListObjectRecords(r.Context(), gen.ListObjectRecordsParams{
		OrganizationID: org, ObjectTypeID: t.ID, Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list records")
		return
	}
	total, err := h.q.CountObjectRecords(r.Context(), gen.CountObjectRecordsParams{OrganizationID: org, ObjectTypeID: t.ID})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not count records")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, rec := range rows {
		items = append(items, recordView(rec))
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "total": total})
}

type recordInput struct {
	Data json.RawMessage `json:"data"`
}

func (h *Handler) createRecord(w http.ResponseWriter, r *http.Request) {
	t, ok := h.resolveType(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	var in recordInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	data := jsonOrEmpty(in.Data)
	if vs, derr := h.recordViolations(r.Context(), t.ID, data); derr != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not load field rules")
		return
	} else if len(vs) > 0 {
		writeViolations(w, vs)
		return
	}
	rec, err := h.q.CreateObjectRecord(r.Context(), gen.CreateObjectRecordParams{
		ObjectTypeID: t.ID, OrganizationID: org, Data: data,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create record")
		return
	}
	audit.SetEntity(r.Context(), "object_records", rec.ID)
	audit.SetSummary(r.Context(), "Created "+t.Code+" record")
	response.JSON(w, http.StatusCreated, recordView(rec))
}

func (h *Handler) getRecord(w http.ResponseWriter, r *http.Request) {
	rec, _, ok := h.loadRecord(w, r)
	if !ok {
		return
	}
	response.JSON(w, http.StatusOK, recordView(rec))
}

func (h *Handler) updateRecord(w http.ResponseWriter, r *http.Request) {
	rec, t, ok := h.loadRecord(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	var in recordInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	data := jsonOrEmpty(in.Data)
	if vs, derr := h.recordViolations(r.Context(), t.ID, data); derr != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not load field rules")
		return
	} else if len(vs) > 0 {
		writeViolations(w, vs)
		return
	}
	upd, err := h.q.UpdateObjectRecord(r.Context(), gen.UpdateObjectRecordParams{OrganizationID: org, ID: rec.ID, Data: data})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not update record")
		return
	}
	audit.SetEntity(r.Context(), "object_records", upd.ID)
	audit.SetSummary(r.Context(), "Updated "+t.Code+" record")
	response.JSON(w, http.StatusOK, recordView(upd))
}

func (h *Handler) deleteRecord(w http.ResponseWriter, r *http.Request) {
	rec, t, ok := h.loadRecord(w, r)
	if !ok {
		return
	}
	org, _ := orgID(r)
	if _, err := h.q.SoftDeleteObjectRecord(r.Context(), gen.SoftDeleteObjectRecordParams{OrganizationID: org, ID: rec.ID}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete record")
		return
	}
	audit.SetEntity(r.Context(), "object_records", rec.ID)
	audit.SetSummary(r.Context(), "Deleted "+t.Code+" record")
	response.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// resolveType maps the {code} path param to the org's object type.
func (h *Handler) resolveType(w http.ResponseWriter, r *http.Request) (gen.ObjectType, bool) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return gen.ObjectType{}, false
	}
	t, err := h.q.GetObjectTypeByCode(r.Context(), gen.GetObjectTypeByCodeParams{OrganizationID: org, Code: chi.URLParam(r, "code")})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "object type not found")
		return gen.ObjectType{}, false
	}
	return t, true
}

// loadRecord resolves both the type (by {code}) and the record (by {id}), and
// confirms the record really belongs to that type — so /objects/supplier/{id}
// can't reach a contract record.
func (h *Handler) loadRecord(w http.ResponseWriter, r *http.Request) (gen.ObjectRecord, gen.ObjectType, bool) {
	t, ok := h.resolveType(w, r)
	if !ok {
		return gen.ObjectRecord{}, gen.ObjectType{}, false
	}
	org, _ := orgID(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return gen.ObjectRecord{}, gen.ObjectType{}, false
	}
	rec, err := h.q.GetObjectRecord(r.Context(), gen.GetObjectRecordParams{OrganizationID: org, ID: id})
	if err != nil || rec.ObjectTypeID != t.ID {
		response.Fail(w, http.StatusNotFound, "not_found", "record not found")
		return gen.ObjectRecord{}, gen.ObjectType{}, false
	}
	return rec, t, true
}

// recordViolations validates a record's data against its type's field rules,
// reusing the product-attribute validation engine.
func (h *Handler) recordViolations(ctx context.Context, typeID int64, data json.RawMessage) ([]validation.Violation, error) {
	fields, err := h.q.ListObjectFieldsForType(ctx, typeID)
	if err != nil {
		return nil, err
	}
	defs := make(map[string]validation.AttrDef, len(fields))
	for _, f := range fields {
		defs[f.Code] = validation.AttrDef{
			Code: f.Code, DataType: f.DataType,
			Options: validation.ParseOptions(f.Options), Rule: validation.ParseRule(f.Validation),
		}
	}
	return validation.ValidateAttributes(defs, data), nil
}

// ---- helpers -------------------------------------------------------------

func typeView(t gen.ObjectType) map[string]any {
	return map[string]any{
		"id": t.ID, "code": t.Code, "label": t.Label, "label_plural": t.LabelPlural,
		"description": t.Description, "is_active": t.IsActive, "created_at": t.CreatedAt.Format(time.RFC3339),
	}
}

func fieldView(f gen.ObjectField) map[string]any {
	return map[string]any{
		"id": f.ID, "object_type_id": f.ObjectTypeID, "code": f.Code, "label": f.Label,
		"data_type": f.DataType, "options": jsonOrNull(f.Options), "validation": jsonOrEmpty(f.Validation),
		"is_required": f.IsRequired, "sort_order": f.SortOrder,
	}
}

func recordView(rec gen.ObjectRecord) map[string]any {
	return map[string]any{
		"id": rec.ID, "public_id": rec.PublicID.String(), "data": jsonOrEmpty(rec.Data),
		"created_at": rec.CreatedAt.Format(time.RFC3339), "updated_at": rec.UpdatedAt.Format(time.RFC3339),
	}
}

func writeViolations(w http.ResponseWriter, vs []validation.Violation) {
	response.JSON(w, http.StatusUnprocessableEntity, map[string]any{
		"code": "validation_failed", "message": "one or more fields failed validation", "violations": vs,
	})
}

func jsonOrNull(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
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
