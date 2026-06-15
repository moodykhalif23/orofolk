package validation

import (
	"encoding/json"
	"testing"
)

func ptrS(s string) *string   { return &s }
func ptrI(i int) *int         { return &i }
func ptrF(f float64) *float64 { return &f }

func defs(ds ...AttrDef) map[string]AttrDef {
	m := make(map[string]AttrDef, len(ds))
	for _, d := range ds {
		m[d.Code] = d
	}
	return m
}

func TestValidateAttributes(t *testing.T) {
	d := defs(
		AttrDef{Code: "sku_code", DataType: "text", Rule: Rule{Pattern: ptrS("^[A-Z]{3}$"), MaxLength: ptrI(3)}},
		AttrDef{Code: "weight", DataType: "number", Rule: Rule{Min: ptrF(0), Max: ptrF(100)}},
		AttrDef{Code: "color", DataType: "select", Options: []string{"red", "blue"}},
		AttrDef{Code: "tags", DataType: "multiselect", Options: []string{"a", "b", "c"}, Rule: Rule{MinSelect: ptrI(1), MaxSelect: ptrI(2)}},
		AttrDef{Code: "active", DataType: "boolean"},
	)

	cases := []struct {
		name     string
		attrs    string
		wantOK   bool
		wantCode string // a code expected among violations when not OK
	}{
		{"all valid", `{"sku_code":"ABC","weight":50,"color":"red","tags":["a","b"],"active":true}`, true, ""},
		{"unknown key ignored", `{"not_an_attr":"whatever"}`, true, ""},
		{"absent/null ignored", `{"weight":null}`, true, ""},
		{"pattern fail", `{"sku_code":"abc"}`, false, "sku_code"},
		{"too long", `{"sku_code":"ABCD"}`, false, "sku_code"},
		{"number over max", `{"weight":150}`, false, "weight"},
		{"number as string ok", `{"weight":"42"}`, true, ""},
		{"number not numeric", `{"weight":"heavy"}`, false, "weight"},
		{"select not allowed", `{"color":"green"}`, false, "color"},
		{"multiselect too many", `{"tags":["a","b","c"]}`, false, "tags"},
		{"multiselect bad option", `{"tags":["z"]}`, false, "tags"},
		{"boolean wrong type", `{"active":"yes"}`, false, "active"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vs := ValidateAttributes(d, json.RawMessage(c.attrs))
			if c.wantOK {
				if len(vs) != 0 {
					t.Fatalf("expected no violations, got %v", vs)
				}
				return
			}
			if len(vs) == 0 {
				t.Fatalf("expected a violation, got none")
			}
			found := false
			for _, v := range vs {
				if v.Code == c.wantCode {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected a violation on %q, got %v", c.wantCode, vs)
			}
		})
	}
}

func TestParseRuleAndOptions(t *testing.T) {
	r := ParseRule([]byte(`{"min":1,"max":9,"pattern":"x"}`))
	if r.Min == nil || *r.Min != 1 || r.Max == nil || *r.Max != 9 || r.Pattern == nil || *r.Pattern != "x" {
		t.Fatalf("ParseRule mismatch: %+v", r)
	}
	if got := ParseRule(nil); got != (Rule{}) {
		t.Fatalf("ParseRule(nil) should be zero, got %+v", got)
	}
	if opts := ParseOptions([]byte(`["red","blue"]`)); len(opts) != 2 || opts[0] != "red" {
		t.Fatalf("ParseOptions mismatch: %v", opts)
	}
	if opts := ParseOptions(nil); opts != nil {
		t.Fatalf("ParseOptions(nil) should be nil, got %v", opts)
	}
}

func TestInvalidPatternIsSkipped(t *testing.T) {
	// An un-compilable regex is a config mistake and must not block writes.
	d := defs(AttrDef{Code: "x", DataType: "text", Rule: Rule{Pattern: ptrS("(")}})
	if vs := ValidateAttributes(d, json.RawMessage(`{"x":"anything"}`)); len(vs) != 0 {
		t.Fatalf("expected no violations for an invalid pattern, got %v", vs)
	}
}
