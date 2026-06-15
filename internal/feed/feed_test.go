package feed

import (
	"encoding/json"
	"strings"
	"testing"
)

var sampleMapping = Mapping{
	{Out: "id", Src: "sku"},
	{Out: "title", Src: "name"},
	{Out: "condition", Const: "new"}, // constant on every row
	{Out: "note", Src: "missing"},    // absent source → empty
}

var sampleRows = []map[string]any{
	{"sku": "A-1", "name": "Widget", "qty": 5.0},
	{"sku": "B-2", "name": "Gad,get", "qty": 10.0}, // comma exercises CSV quoting
}

func TestRenderCSV(t *testing.T) {
	out, err := Render("csv", sampleMapping, sampleRows)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got := string(out)
	if !strings.HasPrefix(got, "id,title,condition,note\n") {
		t.Errorf("header wrong:\n%s", got)
	}
	if !strings.Contains(got, "A-1,Widget,new,\n") {
		t.Errorf("row 1 missing/wrong:\n%s", got)
	}
	// The comma in "Gad,get" must be quoted by the CSV encoder.
	if !strings.Contains(got, `"Gad,get"`) {
		t.Errorf("comma not quoted:\n%s", got)
	}
}

func TestRenderJSON(t *testing.T) {
	out, err := Render("json", sampleMapping, sampleRows)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if len(arr) != 2 {
		t.Fatalf("len=%d, want 2", len(arr))
	}
	if arr[0]["id"] != "A-1" || arr[0]["title"] != "Widget" || arr[0]["condition"] != "new" {
		t.Errorf("row 0 = %v", arr[0])
	}
	// An absent source projects to JSON null.
	if v, ok := arr[0]["note"]; !ok || v != nil {
		t.Errorf("note = %v (ok=%v), want null", v, ok)
	}
}

func TestRenderXML(t *testing.T) {
	m := Mapping{{Out: "g:price", Src: "price"}, {Out: "title", Src: "name"}}
	rows := []map[string]any{{"price": "9.99", "name": "A & B <x>"}}
	out, err := Render("xml", m, rows)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got := string(out)
	// Element name sanitized (":" → "_") and special chars escaped.
	if !strings.Contains(got, "<g_price>9.99</g_price>") {
		t.Errorf("price element wrong:\n%s", got)
	}
	if !strings.Contains(got, "A &amp; B &lt;x&gt;") {
		t.Errorf("text not escaped:\n%s", got)
	}
	if !strings.HasPrefix(got, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("missing xml prolog:\n%s", got)
	}
}

func TestRenderWithRSSEnvelope(t *testing.T) {
	m := Mapping{
		{Out: "g:id", Src: "sku"},
		{Out: "title", Src: "name"},
		{Out: "g:price", Const: "9.99 USD"},
	}
	rows := []map[string]any{{"sku": "A-1", "name": "Widget"}}
	env := &XMLEnvelope{
		Root:      "rss",
		RootAttrs: []XMLAttr{{Name: "version", Value: "2.0"}, {Name: "xmlns:g", Value: "http://base.google.com/ns/1.0"}},
		Channel:   "channel",
		Item:      "item",
		Header:    []XMLAttr{{Name: "title", Value: "Product feed"}},
		Qualified: true,
	}
	out, err := RenderWith("xml", m, rows, RenderOpts{XML: env})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		`<rss version="2.0" xmlns:g="http://base.google.com/ns/1.0">`,
		"<channel>",
		"<title>Product feed</title>",
		"<item>",
		"<g:id>A-1</g:id>", // qualified name preserved under the declared namespace
		"<g:price>9.99 USD</g:price>",
		"</channel>",
		"</rss>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RSS output missing %q:\n%s", want, got)
		}
	}
}

func TestXMLName(t *testing.T) {
	cases := map[string]string{
		"g:price":   "g_price",
		"my field":  "my_field",
		"1leading":  "_1leading",
		"ok-name.1": "ok-name.1",
		"":          "field",
	}
	for in, want := range cases {
		if got := xmlName(in); got != want {
			t.Errorf("xmlName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStr(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"x", "x"},
		{true, "true"},
		{12.5, "12.5"},
		{5.0, "5"},
		{[]any{"a", "b"}, `["a","b"]`},
	}
	for _, c := range cases {
		if got := str(c.in); got != c.want {
			t.Errorf("str(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseMappingAndNormalize(t *testing.T) {
	m := ParseMapping([]byte(`[{"out":"id","src":"sku"},{"out":"c","const":"x"}]`))
	if len(m) != 2 || m[0].Out != "id" || m[0].Src != "sku" || m[1].Const != "x" {
		t.Errorf("parsed mapping = %+v", m)
	}
	if ParseMapping(nil) != nil || ParseMapping([]byte("not json")) != nil {
		t.Error("invalid/empty mapping should parse to nil")
	}
	if NormalizeFormat("xml") != "xml" || NormalizeFormat("weird") != "csv" {
		t.Error("NormalizeFormat clamp wrong")
	}
}
