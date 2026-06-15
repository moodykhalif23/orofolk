// Package feedgen is the feed generation service (Platform roadmap, Phase 4):
// it resolves a feed's data source, projects rows through the channel + mapping
// into a document (via internal/feed), stores the artifact in the blob store and
// runs the scheduled-regeneration sweep. It sits below the HTTP module so the
// background worker can drive the same build path without importing HTTP code —
// mirroring internal/report behind the report-schedule worker.
package feedgen

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"b2bcommerce/internal/store/gen"
)

// ErrUnknownSource is returned when a feed's source key resolves to nothing.
var ErrUnknownSource = errors.New("unknown feed source")

// feedRowCap bounds the rows pulled per generation. A build holds the document
// in memory; this keeps a large catalog from exhausting it. Streaming is a
// future optimization.
const feedRowCap = 50000

// Source is one thing a feed can project FROM: it advertises the field codes a
// mapping may draw on and yields the source rows as flat maps. It mirrors the
// import engine's Target — the same products/object-type duo, read for output.
type Source interface {
	Key() string
	Label() string
	Fields() []string
	Rows(ctx context.Context, q *gen.Queries, org int64, limit int) ([]map[string]any, error)
}

// ResolveSource maps a source key — "products" or "object:<code>" — to a Source.
func ResolveSource(ctx context.Context, q *gen.Queries, org int64, key string) (Source, error) {
	if key == "products" {
		return &productSource{attrCodes: attrCodes(ctx, q, org)}, nil
	}
	if code, ok := strings.CutPrefix(key, "object:"); ok {
		t, err := q.GetObjectTypeByCode(ctx, gen.GetObjectTypeByCodeParams{OrganizationID: org, Code: code})
		if err != nil {
			return nil, ErrUnknownSource
		}
		fields, err := q.ListObjectFieldsForType(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		return &objectSource{typ: t, fields: fields}, nil
	}
	return nil, ErrUnknownSource
}

// AvailableSources lists everything projectable for an org: products + one
// source per custom object type.
func AvailableSources(ctx context.Context, q *gen.Queries, org int64) ([]Source, error) {
	out := []Source{&productSource{attrCodes: attrCodes(ctx, q, org)}}
	types, err := q.ListObjectTypes(ctx, org)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		fields, err := q.ListObjectFieldsForType(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, &objectSource{typ: t, fields: fields})
	}
	return out, nil
}

func attrCodes(ctx context.Context, q *gen.Queries, org int64) []string {
	attrs, err := q.ListAttributes(ctx, org)
	if err != nil {
		return nil
	}
	codes := make([]string, len(attrs))
	for i, a := range attrs {
		codes[i] = a.Code
	}
	return codes
}

// ---- products source ------------------------------------------------------

// productBaseFields are the structural product columns a feed may map (besides
// the dynamic attr.<code> fields).
var productBaseFields = []string{"id", "sku", "name", "slug", "description", "status", "type", "unit", "image_url", "created_at"}

type productSource struct{ attrCodes []string }

func (p *productSource) Key() string   { return "products" }
func (p *productSource) Label() string { return "Products" }
func (p *productSource) Fields() []string {
	out := make([]string, 0, len(productBaseFields)+len(p.attrCodes))
	out = append(out, productBaseFields...)
	for _, c := range p.attrCodes {
		out = append(out, "attr."+c)
	}
	return out
}

func (p *productSource) Rows(ctx context.Context, q *gen.Queries, org int64, limit int) ([]map[string]any, error) {
	rows, err := q.ListProductsForFeed(ctx, gen.ListProductsForFeedParams{OrganizationID: org, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m := map[string]any{
			"id": r.PublicID.String(), "sku": r.Sku, "name": r.Name, "slug": r.Slug,
			"description": deref(r.Description), "status": r.Status, "type": r.Type, "unit": r.Unit,
			"image_url": r.ImageUrl, "created_at": r.CreatedAt.UTC().Format(time.RFC3339),
		}
		// Flatten attributes to attr.<code> so a mapping can pull any of them.
		var attrs map[string]any
		if len(r.Attributes) > 0 && json.Unmarshal(r.Attributes, &attrs) == nil {
			for k, v := range attrs {
				m["attr."+k] = v
			}
		}
		out = append(out, m)
	}
	return out, nil
}

// ---- custom object records source -----------------------------------------

type objectSource struct {
	typ    gen.ObjectType
	fields []gen.ObjectField
}

func (o *objectSource) Key() string   { return "object:" + o.typ.Code }
func (o *objectSource) Label() string { return o.typ.Label }
func (o *objectSource) Fields() []string {
	out := make([]string, 0, len(o.fields)+2)
	out = append(out, "id", "created_at")
	for _, f := range o.fields {
		out = append(out, f.Code)
	}
	return out
}

func (o *objectSource) Rows(ctx context.Context, q *gen.Queries, org int64, limit int) ([]map[string]any, error) {
	recs, err := q.ListObjectRecords(ctx, gen.ListObjectRecordsParams{
		OrganizationID: org, ObjectTypeID: o.typ.ID, Limit: int32(limit), Offset: 0,
	})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(recs))
	for _, rec := range recs {
		m := map[string]any{"id": rec.PublicID.String(), "created_at": rec.CreatedAt.UTC().Format(time.RFC3339)}
		var data map[string]any
		if len(rec.Data) > 0 && json.Unmarshal(rec.Data, &data) == nil {
			for k, v := range data {
				m[k] = v
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
