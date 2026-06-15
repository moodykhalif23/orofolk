package imports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/validation"
)

var errUnknownTarget = errors.New("unknown import target")

// Verdict is the dry-run outcome for one parsed row: how it will be applied
// (create/update) or why it can't be (error), plus the normalized data that a
// commit will persist.
type Verdict struct {
	Status  string          // "create" | "update" | "error"
	Data    json.RawMessage // normalized row, stored on import_rows and applied on commit
	Message string
}

// FieldSpec describes one importable column of a target: its key, type, whether
// it's required, the allowed values (select types) and a short rule summary. It
// is the machine-readable "template" a partner integration codes against —
// richer than the bare column list the CSV header carries.
type FieldSpec struct {
	Code     string   `json:"code"`
	Label    string   `json:"label,omitempty"`
	DataType string   `json:"data_type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
	Rule     string   `json:"rule,omitempty"`
}

// Target is one thing the engine can import into. Plan validates and classifies
// a parsed row without writing; Apply persists a planned row on commit. Both
// take a *gen.Queries so the same target works inside or outside a transaction.
type Target interface {
	Key() string
	Label() string
	Columns() []string
	Schema() []FieldSpec
	Plan(ctx context.Context, q *gen.Queries, org int64, row map[string]any) Verdict
	Apply(ctx context.Context, q *gen.Queries, org int64, data json.RawMessage, status string) error
}

// resolveTarget maps a target key — "products" or "object:<code>" — to a Target.
func resolveTarget(ctx context.Context, q *gen.Queries, org int64, key, matchField string) (Target, error) {
	if key == "products" {
		return &productTarget{}, nil
	}
	if code, ok := strings.CutPrefix(key, "object:"); ok {
		t, err := q.GetObjectTypeByCode(ctx, gen.GetObjectTypeByCodeParams{OrganizationID: org, Code: code})
		if err != nil {
			return nil, errUnknownTarget
		}
		fields, err := q.ListObjectFieldsForType(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		return &objectTarget{typ: t, fields: fields, matchField: matchField}, nil
	}
	return nil, errUnknownTarget
}

// availableTargets lists everything importable for an org: products + one target
// per custom object type.
func availableTargets(ctx context.Context, q *gen.Queries, org int64) ([]Target, error) {
	out := []Target{&productTarget{}}
	types, err := q.ListObjectTypes(ctx, org)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		fields, err := q.ListObjectFieldsForType(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, &objectTarget{typ: t, fields: fields})
	}
	return out, nil
}

// ---- products target -----------------------------------------------------

type productTarget struct{}

func (p *productTarget) Key() string   { return "products" }
func (p *productTarget) Label() string { return "Products" }
func (p *productTarget) Columns() []string {
	return []string{"sku", "name", "slug", "type", "status", "unit", "cost_price", "description", "attributes"}
}

// Schema describes the product columns. sku/name/slug are required (enforced in
// Plan); the rest default when omitted. `attributes` is a JSON object validated
// against the org's attribute rules.
func (p *productTarget) Schema() []FieldSpec {
	return []FieldSpec{
		{Code: "sku", Label: "SKU", DataType: "text", Required: true},
		{Code: "name", Label: "Name", DataType: "text", Required: true},
		{Code: "slug", Label: "Slug", DataType: "text", Required: true},
		{Code: "type", Label: "Type", DataType: "text", Rule: "defaults to simple"},
		{Code: "status", Label: "Status", DataType: "text", Rule: "defaults to draft"},
		{Code: "unit", Label: "Unit", DataType: "text", Rule: "defaults to each"},
		{Code: "cost_price", Label: "Cost price", DataType: "price", Rule: "defaults to 0"},
		{Code: "description", Label: "Description", DataType: "text"},
		{Code: "attributes", Label: "Attributes", DataType: "json", Rule: "JSON object, validated against attribute rules"},
	}
}

type productData struct {
	Sku         string          `json:"sku"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Type        string          `json:"type"`
	Status      string          `json:"status"`
	Unit        string          `json:"unit"`
	CostPrice   string          `json:"cost_price"`
	Description string          `json:"description"`
	Attributes  json.RawMessage `json:"attributes"`
}

func (p *productTarget) Plan(ctx context.Context, q *gen.Queries, org int64, row map[string]any) Verdict {
	get := func(k string) string { return strings.TrimSpace(asString(row[k])) }
	sku, name, slug := get("sku"), get("name"), get("slug")
	if sku == "" || name == "" || slug == "" {
		return Verdict{Status: "error", Message: "sku, name and slug are required"}
	}
	attrs := json.RawMessage("{}")
	if raw, ok := rawJSONField(row, "attributes"); ok {
		if !json.Valid(raw) {
			return Verdict{Status: "error", Message: "attributes is not valid JSON"}
		}
		attrs = raw
	}
	// Same attribute rules the live product write enforces.
	if defs, err := attrDefs(ctx, q, org); err == nil {
		if vs := validation.ValidateAttributes(defs, attrs); len(vs) > 0 {
			return Verdict{Status: "error", Message: vs[0].Code + ": " + vs[0].Message}
		}
	}
	pd := productData{
		Sku: sku, Name: name, Slug: slug,
		Type: orDefault(get("type"), "simple"), Status: orDefault(get("status"), "draft"),
		Unit: orDefault(get("unit"), "each"), CostPrice: orDefault(get("cost_price"), "0"),
		Description: get("description"), Attributes: attrs,
	}
	data, _ := json.Marshal(pd)
	status := "create"
	if _, err := q.GetProductBySKU(ctx, gen.GetProductBySKUParams{OrganizationID: org, Sku: sku}); err == nil {
		status = "update"
	}
	return Verdict{Status: status, Data: data}
}

func (p *productTarget) Apply(ctx context.Context, q *gen.Queries, org int64, data json.RawMessage, status string) error {
	var pd productData
	if err := json.Unmarshal(data, &pd); err != nil {
		return err
	}
	var desc *string
	if pd.Description != "" {
		desc = &pd.Description
	}
	attrs := []byte(pd.Attributes)
	if len(attrs) == 0 {
		attrs = []byte("{}")
	}
	if status == "update" {
		existing, err := q.GetProductBySKU(ctx, gen.GetProductBySKUParams{OrganizationID: org, Sku: pd.Sku})
		if err != nil {
			return err
		}
		up, err := q.UpdateProduct(ctx, gen.UpdateProductParams{
			OrganizationID: org, ID: existing.ID, Sku: pd.Sku, Type: pd.Type, Name: pd.Name, Slug: pd.Slug,
			Description: desc, Status: pd.Status, Attributes: attrs, Unit: pd.Unit,
		})
		if err != nil {
			return err
		}
		_, err = q.SetProductCost(ctx, gen.SetProductCostParams{OrganizationID: org, ID: up.ID, CostPrice: pd.CostPrice})
		return err
	}
	created, err := q.CreateProduct(ctx, gen.CreateProductParams{
		OrganizationID: org, Sku: pd.Sku, Type: pd.Type, Name: pd.Name, Slug: pd.Slug,
		Description: desc, Status: pd.Status, Attributes: attrs, Unit: pd.Unit,
	})
	if err != nil {
		return err
	}
	_, err = q.SetProductCost(ctx, gen.SetProductCostParams{OrganizationID: org, ID: created.ID, CostPrice: pd.CostPrice})
	return err
}

// ---- custom object records target ----------------------------------------

type objectTarget struct {
	typ        gen.ObjectType
	fields     []gen.ObjectField
	matchField string // when set, upsert: match an existing record on this field's value
}

func (o *objectTarget) Key() string   { return "object:" + o.typ.Code }
func (o *objectTarget) Label() string { return o.typ.Label }
func (o *objectTarget) Columns() []string {
	cols := make([]string, len(o.fields))
	for i, f := range o.fields {
		cols[i] = f.Code
	}
	return cols
}

// Schema describes each defined field — type, required flag, allowed values and
// a rule summary — derived from the same field definitions Plan validates with.
func (o *objectTarget) Schema() []FieldSpec {
	out := make([]FieldSpec, len(o.fields))
	for i, f := range o.fields {
		out[i] = FieldSpec{
			Code:     f.Code,
			Label:    f.Label,
			DataType: f.DataType,
			Required: f.IsRequired,
			Options:  validation.ParseOptions(f.Options),
			Rule:     ruleSummary(validation.ParseRule(f.Validation)),
		}
	}
	return out
}

func (o *objectTarget) Plan(ctx context.Context, q *gen.Queries, org int64, row map[string]any) Verdict {
	data := map[string]any{}
	defs := make(map[string]validation.AttrDef, len(o.fields))
	for _, f := range o.fields {
		defs[f.Code] = validation.AttrDef{
			Code: f.Code, DataType: f.DataType,
			Options: validation.ParseOptions(f.Options), Rule: validation.ParseRule(f.Validation),
		}
		if raw, ok := row[f.Code]; ok {
			if v := coerce(f.DataType, raw); v != nil {
				data[f.Code] = v
			}
		}
	}
	jb, _ := json.Marshal(data)
	if vs := validation.ValidateAttributes(defs, jb); len(vs) > 0 {
		return Verdict{Status: "error", Data: jb, Message: vs[0].Code + ": " + vs[0].Message}
	}
	if o.findMatch(ctx, q, org, data) > 0 {
		return Verdict{Status: "update", Data: jb}
	}
	return Verdict{Status: "create", Data: jb}
}

func (o *objectTarget) Apply(ctx context.Context, q *gen.Queries, org int64, data json.RawMessage, status string) error {
	if status == "update" {
		var m map[string]any
		if json.Unmarshal(data, &m) == nil {
			if id := o.findMatch(ctx, q, org, m); id > 0 {
				_, err := q.UpdateObjectRecord(ctx, gen.UpdateObjectRecordParams{OrganizationID: org, ID: id, Data: []byte(data)})
				return err
			}
		}
	}
	_, err := q.CreateObjectRecord(ctx, gen.CreateObjectRecordParams{
		ObjectTypeID: o.typ.ID, OrganizationID: org, Data: []byte(data),
	})
	return err
}

// findMatch returns the id of an existing record whose match-field value equals
// this row's, or 0 when there's no match field, no value, or no match.
func (o *objectTarget) findMatch(ctx context.Context, q *gen.Queries, org int64, data map[string]any) int64 {
	if o.matchField == "" {
		return 0
	}
	v, ok := data[o.matchField]
	if !ok {
		return 0
	}
	id, err := q.GetObjectRecordIDByField(ctx, gen.GetObjectRecordIDByFieldParams{
		OrganizationID: org, ObjectTypeID: o.typ.ID, Field: o.matchField, Value: fmt.Sprint(v),
	})
	if err != nil {
		return 0
	}
	return id
}

// ---- shared helpers ------------------------------------------------------

func attrDefs(ctx context.Context, q *gen.Queries, org int64) (map[string]validation.AttrDef, error) {
	rows, err := q.ListAttributes(ctx, org)
	if err != nil {
		return nil, err
	}
	defs := make(map[string]validation.AttrDef, len(rows))
	for _, a := range rows {
		defs[a.Code] = validation.AttrDef{
			Code: a.Code, DataType: a.DataType,
			Options: validation.ParseOptions(a.Options), Rule: validation.ParseRule(a.Validation),
		}
	}
	return defs, nil
}

// coerce turns a raw cell (string from CSV, typed value from JSON) into the JSON
// shape a field of the given type expects. A bad value is left as-is so the
// validation engine reports it precisely.
func coerce(dataType string, raw any) any {
	switch dataType {
	case "number", "price":
		if f, ok := raw.(float64); ok {
			return f
		}
		s := strings.TrimSpace(asString(raw))
		if s == "" {
			return nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	case "boolean":
		if b, ok := raw.(bool); ok {
			return b
		}
		s := strings.TrimSpace(asString(raw))
		if s == "" {
			return nil
		}
		b, _ := strconv.ParseBool(s)
		return b
	case "multiselect":
		if arr, ok := raw.([]any); ok {
			return arr
		}
		s := strings.TrimSpace(asString(raw))
		if s == "" {
			return nil
		}
		out := []any{}
		for _, p := range strings.Split(s, ";") {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out
	default: // text, select, date, file
		s := strings.TrimSpace(asString(raw))
		if s == "" {
			return nil
		}
		return s
	}
}

// rawJSONField returns a field as raw JSON: a string cell (CSV) is used verbatim;
// a structured value (JSON upload) is re-marshalled.
func rawJSONField(row map[string]any, key string) (json.RawMessage, bool) {
	v, ok := row[key]
	if !ok || v == nil {
		return nil, false
	}
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, false
		}
		return json.RawMessage(s), true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	return b, true
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ruleSummary renders a validation.Rule as a short human phrase for a target's
// schema (empty when the rule constrains nothing).
func ruleSummary(r validation.Rule) string {
	num := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	var parts []string
	if r.Pattern != nil && *r.Pattern != "" {
		parts = append(parts, "matches /"+*r.Pattern+"/")
	}
	switch {
	case r.MinLength != nil && r.MaxLength != nil:
		parts = append(parts, fmt.Sprintf("%d–%d chars", *r.MinLength, *r.MaxLength))
	case r.MinLength != nil:
		parts = append(parts, fmt.Sprintf("≥ %d chars", *r.MinLength))
	case r.MaxLength != nil:
		parts = append(parts, fmt.Sprintf("≤ %d chars", *r.MaxLength))
	}
	switch {
	case r.Min != nil && r.Max != nil:
		parts = append(parts, num(*r.Min)+"–"+num(*r.Max))
	case r.Min != nil:
		parts = append(parts, "≥ "+num(*r.Min))
	case r.Max != nil:
		parts = append(parts, "≤ "+num(*r.Max))
	}
	switch {
	case r.MinSelect != nil && r.MaxSelect != nil:
		parts = append(parts, fmt.Sprintf("%d–%d selections", *r.MinSelect, *r.MaxSelect))
	case r.MinSelect != nil:
		parts = append(parts, fmt.Sprintf("≥ %d selections", *r.MinSelect))
	case r.MaxSelect != nil:
		parts = append(parts, fmt.Sprintf("≤ %d selections", *r.MaxSelect))
	}
	return strings.Join(parts, ", ")
}

// importOptions are per-run import settings, stored on the run so a commit
// re-applies the same matching the dry run used.
type importOptions struct {
	MatchField string   `json:"match_field,omitempty"`
	Normalize  []string `json:"normalize,omitempty"`
}

// normalizeRows cleanses string cell values in place per the configured rules,
// before any target sees them.
func normalizeRows(rows []map[string]any, ns []string) {
	if len(ns) == 0 {
		return
	}
	for _, row := range rows {
		for k, v := range row {
			if s, ok := v.(string); ok {
				row[k] = applyNorm(ns, s)
			}
		}
	}
}

func applyNorm(ns []string, s string) string {
	for _, n := range ns {
		switch n {
		case "trim":
			s = strings.TrimSpace(s)
		case "lower":
			s = strings.ToLower(s)
		case "upper":
			s = strings.ToUpper(s)
		case "collapse":
			s = strings.Join(strings.Fields(s), " ")
		}
	}
	return s
}
