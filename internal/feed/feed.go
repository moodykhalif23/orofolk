// Package feed projects mapped data rows into a syndication document — CSV,
// JSON or XML (Platform roadmap, Phase 4). It is a pure encoder layer (no
// database): a caller supplies a Mapping (an ordered list of output columns,
// each drawn from a source field or a constant) and the source rows as
// map[string]any, and Render returns the encoded bytes. It is the outbound twin
// of the import engine — data shaped on the way OUT through a mapping, as
// imports validate it on the way IN.
package feed

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"

	"b2bcommerce/internal/export"
)

// Field is one output column: Out is its name in the document; the value is the
// Const literal when set, otherwise the source row's Src field (empty if
// absent). Exactly the shape stored on feeds.mapping.
type Field struct {
	Out   string `json:"out"`
	Src   string `json:"src,omitempty"`
	Const string `json:"const,omitempty"`
}

// Mapping is the ordered projection a feed applies to every source row.
type Mapping []Field

// ParseMapping decodes a feeds.mapping JSONB column (nil/invalid → empty).
func ParseMapping(raw []byte) Mapping {
	if len(raw) == 0 {
		return nil
	}
	var m Mapping
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

// Formats lists the document formats Render supports.
func Formats() []string { return []string{"csv", "json", "xml"} }

// NormalizeFormat clamps an arbitrary string to a supported format (default csv).
func NormalizeFormat(s string) string {
	switch s {
	case "json", "xml", "csv":
		return s
	default:
		return "csv"
	}
}

// ContentType is the MIME type for a rendered feed of the given format.
func ContentType(format string) string {
	switch NormalizeFormat(format) {
	case "json":
		return "application/json; charset=utf-8"
	case "xml":
		return "application/xml; charset=utf-8"
	default:
		return "text/csv; charset=utf-8"
	}
}

// Ext is the filename extension for a rendered feed (no dot).
func Ext(format string) string { return NormalizeFormat(format) }

// value resolves one field for one row: the constant when set, else the named
// source value (nil when the field has neither a source nor a constant).
func (f Field) value(row map[string]any) any {
	if f.Const != "" {
		return f.Const
	}
	if f.Src == "" {
		return nil
	}
	return row[f.Src]
}

// XMLAttr is a name/value pair — a root attribute (namespace/version) or a
// constant header element. Ordered, so output is deterministic.
type XMLAttr struct{ Name, Value string }

// XMLEnvelope customizes XML output so a channel adapter can emit a destination's
// document shape (e.g. Google Shopping's RSS 2.0 with the g: namespace). The zero
// value renders the generic <feed><item>…</item></feed>.
type XMLEnvelope struct {
	Root      string    // root element (default "feed")
	RootAttrs []XMLAttr // ordered root attributes, e.g. version + xmlns:g
	Channel   string    // optional wrapper element inside root (e.g. RSS "channel"); "" = none
	Item      string    // per-row element (default "item")
	Header    []XMLAttr // constant elements emitted once before items (e.g. RSS <title>)
	Qualified bool       // keep "prefix:name" element names (a declared namespace makes them valid)
}

func (e XMLEnvelope) withDefaults() XMLEnvelope {
	if e.Root == "" {
		e.Root = "feed"
	}
	if e.Item == "" {
		e.Item = "item"
	}
	return e
}

// RenderOpts carries optional per-render configuration. A nil XML envelope uses
// the generic shape.
type RenderOpts struct{ XML *XMLEnvelope }

// Render projects rows through the mapping into the named format (generic shape).
func Render(format string, m Mapping, rows []map[string]any) ([]byte, error) {
	return RenderWith(format, m, rows, RenderOpts{})
}

// RenderWith is Render with channel customization (currently the XML envelope).
func RenderWith(format string, m Mapping, rows []map[string]any, opts RenderOpts) ([]byte, error) {
	switch NormalizeFormat(format) {
	case "json":
		return renderJSON(m, rows)
	case "xml":
		env := XMLEnvelope{}.withDefaults()
		if opts.XML != nil {
			env = opts.XML.withDefaults()
		}
		return renderXML(m, rows, env)
	default:
		return renderCSV(m, rows)
	}
}

func renderCSV(m Mapping, rows []map[string]any) ([]byte, error) {
	cols := make([]export.Column, len(m))
	for i, f := range m {
		cols[i] = export.Column{Name: f.Out}
	}
	var buf bytes.Buffer
	s, err := export.NewCSVStream(&buf, cols)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		cells := make([]string, len(m))
		for i, f := range m {
			cells[i] = str(f.value(row))
		}
		if err := s.Write(cells); err != nil {
			return nil, err
		}
	}
	if err := s.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderJSON(m Mapping, rows []map[string]any) ([]byte, error) {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		obj := make(map[string]any, len(m))
		for _, f := range m {
			obj[f.Out] = f.value(row)
		}
		out = append(out, obj)
	}
	return json.MarshalIndent(out, "", "  ")
}

func renderXML(m Mapping, rows []map[string]any, env XMLEnvelope) ([]byte, error) {
	nameFn := xmlName
	if env.Qualified {
		nameFn = qualifiedXMLName
	}
	// Pre-compute element names once (stable per mapping).
	names := make([]string, len(m))
	for i, f := range m {
		names[i] = nameFn(f.Out)
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString("<" + env.Root)
	for _, a := range env.RootAttrs {
		buf.WriteString(" " + a.Name + `="`)
		if err := xml.EscapeText(&buf, []byte(a.Value)); err != nil {
			return nil, err
		}
		buf.WriteString(`"`)
	}
	buf.WriteString(">\n")

	itemIndent := "  "
	if env.Channel != "" {
		buf.WriteString("  <" + env.Channel + ">\n")
		itemIndent = "    "
	}
	// Constant header elements (e.g. RSS channel <title>/<link>/<description>).
	for _, h := range env.Header {
		nm := nameFn(h.Name)
		buf.WriteString(itemIndent + "<" + nm + ">")
		if err := xml.EscapeText(&buf, []byte(h.Value)); err != nil {
			return nil, err
		}
		buf.WriteString("</" + nm + ">\n")
	}
	cellIndent := itemIndent + "  "
	for _, row := range rows {
		buf.WriteString(itemIndent + "<" + env.Item + ">\n")
		for i, f := range m {
			buf.WriteString(cellIndent + "<" + names[i] + ">")
			if err := xml.EscapeText(&buf, []byte(str(f.value(row)))); err != nil {
				return nil, err
			}
			buf.WriteString("</" + names[i] + ">\n")
		}
		buf.WriteString(itemIndent + "</" + env.Item + ">\n")
	}
	if env.Channel != "" {
		buf.WriteString("  </" + env.Channel + ">\n")
	}
	buf.WriteString("</" + env.Root + ">\n")
	return buf.Bytes(), nil
}

// str renders a projected value as a scalar string for CSV/XML cells. Scalars
// are formatted plainly; composite values (arrays/objects) fall back to compact
// JSON so nothing is silently dropped.
func str(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case json.Number:
		return x.String()
	default:
		if b, err := json.Marshal(x); err == nil {
			return string(b)
		}
		return fmt.Sprint(x)
	}
}

// xmlName sanitizes an output column name into a valid XML element name: it
// keeps letters, digits, '-', '_' and '.', maps anything else (spaces, ':' from
// e.g. "g:price") to '_', and ensures a legal first character. Channel presets
// (a later slice) supply proper namespaced names; this keeps generic XML valid.
func xmlName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "field"
	}
	if c := out[0]; !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		out = append([]rune{'_'}, out...)
	}
	return string(out)
}

// qualifiedXMLName is xmlName that additionally keeps ':' so a namespaced name
// like "g:price" survives — valid when the prefix's namespace is declared on the
// root (which the channel envelope does). Used only when the envelope opts in.
func qualifiedXMLName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == ':':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "field"
	}
	if c := out[0]; !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		out = append([]rune{'_'}, out...)
	}
	return string(out)
}

// FieldsSorted returns the distinct source-field names a mapping references,
// sorted — handy for validating a mapping against a source's available fields.
func (m Mapping) FieldsSorted() []string {
	seen := map[string]struct{}{}
	for _, f := range m {
		if f.Src != "" {
			seen[f.Src] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
