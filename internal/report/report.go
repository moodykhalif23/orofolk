// Package report is the safe report compiler (Pack 3 §1.5). Analysts never write
// SQL: a ReportDefinition (entity + dimensions + measures + filters) is compiled
// against per-entity allow-lists into parameterized SQL. Column names come only
// from the allow-list (never interpolated user input); values are always bound
// parameters; unknown fields are rejected at save time. Every selected column is
// cast to text so result scanning and CSV/JSON serialization are uniform.
package report

import (
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Measure is an aggregation over a measurable field. Field "" or "*" with
// agg "count" compiles to count(*).
type Measure struct {
	Field string `json:"field"`
	Agg   string `json:"agg"`
}

// Filter constrains rows. Op is one of eq, ne, gt, gte, lt, lte.
type Filter struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// Definition is the constrained, user-authored report model.
type Definition struct {
	Entity     string    `json:"entity"`
	Dimensions []string  `json:"dimensions"`
	Measures   []Measure `json:"measures"`
	Filters    []Filter  `json:"filters"`
	Limit      int       `json:"limit"`
}

const maxRows = 10000

var allowedAgg = map[string]bool{"sum": true, "avg": true, "count": true, "min": true, "max": true}

var ops = map[string]string{"eq": "=", "ne": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}

type filterSpec struct {
	col  string
	kind string // text | num | date
}

type entitySpec struct {
	from     string // FROM clause incl. fixed joins (no user input)
	orgCol   string
	dims     map[string]string // name -> SQL expr
	measures map[string]string // field -> SQL column
	filters  map[string]filterSpec
}

// dateDims are the shared date-trunc dimensions, parameterized by the entity's
// created_at column.
func dateDims(createdAt string) map[string]string {
	return map[string]string{
		"day":   "date_trunc('day', " + createdAt + ")::date",
		"week":  "date_trunc('week', " + createdAt + ")::date",
		"month": "date_trunc('month', " + createdAt + ")::date",
	}
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// registry is the entire safe surface: only these entities/fields are reachable.
var registry = map[string]entitySpec{
	"orders": {
		from:   "orders o",
		orgCol: "o.organization_id",
		dims:   merge(dateDims("o.created_at"), map[string]string{"status": "o.status", "currency": "o.currency", "customer": "o.customer_id"}),
		measures: map[string]string{
			"grand_total": "o.grand_total", "subtotal": "o.subtotal", "tax_total": "o.tax_total",
		},
		filters: map[string]filterSpec{
			"status": {"o.status", "text"}, "currency": {"o.currency", "text"},
			"created_at": {"o.created_at", "date"}, "customer": {"o.customer_id", "num"},
		},
	},
	"invoices": {
		from:   "invoices i JOIN orders o ON o.id = i.order_id",
		orgCol: "o.organization_id",
		dims:   merge(dateDims("i.created_at"), map[string]string{"status": "i.status", "currency": "i.currency"}),
		measures: map[string]string{
			"grand_total": "i.grand_total", "subtotal": "i.subtotal", "tax_total": "i.tax_total",
		},
		filters: map[string]filterSpec{
			"status": {"i.status", "text"}, "currency": {"i.currency", "text"}, "created_at": {"i.created_at", "date"},
		},
	},
	"opportunities": {
		from:     "opportunities op JOIN pipeline_stages ps ON ps.id = op.stage_id",
		orgCol:   "op.organization_id",
		dims:     merge(dateDims("op.created_at"), map[string]string{"stage": "ps.name", "currency": "op.currency"}),
		measures: map[string]string{"amount": "op.amount"},
		filters: map[string]filterSpec{
			"stage": {"ps.name", "text"}, "currency": {"op.currency", "text"}, "created_at": {"op.created_at", "date"},
		},
	},
}

// EntitySchema is the serializable surface of one entity for the builder UI.
type EntitySchema struct {
	Dimensions []string `json:"dimensions"`
	Measures   []string `json:"measures"`
	Filters    []string `json:"filters"`
}

// Schema returns the allow-listed dimensions/measures/filters per entity, so the
// builder UI can offer only valid choices (the same allow-list the compiler
// enforces). count(*) is always an available measure.
func Schema() map[string]EntitySchema {
	out := map[string]EntitySchema{}
	for name, spec := range registry {
		es := EntitySchema{}
		for d := range spec.dims {
			es.Dimensions = append(es.Dimensions, d)
		}
		es.Measures = append(es.Measures, "count")
		for m := range spec.measures {
			es.Measures = append(es.Measures, m)
		}
		for f := range spec.filters {
			es.Filters = append(es.Filters, f)
		}
		sort.Strings(es.Dimensions)
		sort.Strings(es.Measures)
		sort.Strings(es.Filters)
		out[name] = es
	}
	return out
}

// Compiled is a ready-to-execute query plus its output column names.
type Compiled struct {
	SQL  string
	Args []any
	Cols []string
}

// Compile turns a definition into safe, parameterized SQL scoped to org. It
// errors on any field/agg/op not in the entity's allow-list — callers run this
// at save time so bad definitions are rejected before they can ever execute.
func Compile(org int64, def Definition) (Compiled, error) {
	spec, ok := registry[def.Entity]
	if !ok {
		return Compiled{}, fmt.Errorf("unknown entity %q", def.Entity)
	}
	if len(def.Measures) == 0 {
		return Compiled{}, fmt.Errorf("at least one measure is required")
	}

	var selects, groupBys, cols []string

	for _, d := range def.Dimensions {
		expr, ok := spec.dims[d]
		if !ok {
			return Compiled{}, fmt.Errorf("unknown dimension %q for %s", d, def.Entity)
		}
		selects = append(selects, "("+expr+")::text AS "+ident(d))
		groupBys = append(groupBys, expr)
		cols = append(cols, d)
	}

	var orderBy string
	for i, m := range def.Measures {
		if !allowedAgg[m.Agg] {
			return Compiled{}, fmt.Errorf("unknown aggregation %q", m.Agg)
		}
		var expr, alias string
		if m.Agg == "count" && (m.Field == "" || m.Field == "*") {
			expr, alias = "count(*)", "count"
		} else {
			col, ok := spec.measures[m.Field]
			if !ok {
				return Compiled{}, fmt.Errorf("unknown measure %q for %s", m.Field, def.Entity)
			}
			expr = m.Agg + "(" + col + ")"
			alias = m.Field + "_" + m.Agg
		}
		selects = append(selects, "("+expr+")::text AS "+ident(alias))
		cols = append(cols, alias)
		if i == 0 {
			orderBy = expr + " DESC"
		}
	}

	args := []any{org}
	where := []string{spec.orgCol + " = $1"}
	for _, f := range def.Filters {
		fs, ok := spec.filters[f.Field]
		if !ok {
			return Compiled{}, fmt.Errorf("unknown filter field %q for %s", f.Field, def.Entity)
		}
		sqlOp, ok := ops[f.Op]
		if !ok {
			return Compiled{}, fmt.Errorf("unknown filter op %q", f.Op)
		}
		val, err := coerce(fs.kind, f.Value)
		if err != nil {
			return Compiled{}, fmt.Errorf("filter %q: %w", f.Field, err)
		}
		args = append(args, val)
		where = append(where, fmt.Sprintf("%s %s $%d", fs.col, sqlOp, len(args)))
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(selects, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(spec.from)
	sb.WriteString(" WHERE ")
	sb.WriteString(strings.Join(where, " AND "))
	if len(groupBys) > 0 {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(groupBys, ", "))
	}
	if orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(orderBy)
	}
	limit := def.Limit
	if limit <= 0 || limit > maxRows {
		limit = maxRows
	}
	sb.WriteString(" LIMIT ")
	sb.WriteString(strconv.Itoa(limit))

	return Compiled{SQL: sb.String(), Args: args, Cols: cols}, nil
}

// Run compiles and executes the definition, returning columns and string rows
// (NULLs as nil). Every column is text (cast in Compile), so scanning is uniform.
func Run(ctx context.Context, pool *pgxpool.Pool, org int64, def Definition) ([]string, [][]*string, error) {
	c, err := Compile(org, def)
	if err != nil {
		return nil, nil, err
	}
	rows, err := pool.Query(ctx, c.SQL, c.Args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var out [][]*string
	for rows.Next() {
		cells := make([]*string, len(c.Cols))
		ptrs := make([]any, len(c.Cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		out = append(out, cells)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return c.Cols, out, nil
}

// ToCSV renders columns + rows as CSV bytes (NULLs as empty cells).
func ToCSV(cols []string, rows [][]*string) ([]byte, error) {
	var sb strings.Builder
	cw := csv.NewWriter(&sb)
	if err := cw.Write(cols); err != nil {
		return nil, err
	}
	rec := make([]string, len(cols))
	for _, r := range rows {
		for i, c := range r {
			if c == nil {
				rec[i] = ""
			} else {
				rec[i] = *c
			}
		}
		if err := cw.Write(rec); err != nil {
			return nil, err
		}
	}
	cw.Flush()
	return []byte(sb.String()), cw.Error()
}

// coerce converts a JSON filter value to a type the column comparison accepts.
func coerce(kind string, v any) (any, error) {
	switch kind {
	case "date":
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("date value must be a string")
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, nil
		}
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil, fmt.Errorf("invalid date %q", s)
		}
		return t, nil
	case "num":
		switch n := v.(type) {
		case float64:
			if n == float64(int64(n)) {
				return int64(n), nil // integer columns (e.g. customer_id) need int
			}
			return n, nil
		case string:
			if i, err := strconv.ParseInt(n, 10, 64); err == nil {
				return i, nil
			}
			f, err := strconv.ParseFloat(n, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number %q", n)
			}
			return f, nil
		default:
			return nil, fmt.Errorf("numeric value required")
		}
	default: // text
		if s, ok := v.(string); ok {
			return s, nil
		}
		return fmt.Sprint(v), nil
	}
}

// ident validates an alias is a safe SQL identifier (allow-list keys already
// are; this is defense-in-depth) and returns it.
func ident(s string) string {
	for _, r := range s {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return "col" // never reached for allow-listed keys
		}
	}
	return s
}
