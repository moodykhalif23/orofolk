// Package validation enforces per-attribute data-quality rules (Platform
// roadmap, Phase 1). A value stored in a product's attributes JSONB must satisfy
// the constraints its attribute definition declares: allowed values (select /
// multiselect, from `options`), regex + length (text), numeric range
// (number / price) and selection count (multiselect). The same engine runs on
// product writes and on CSV import, so a bad value is rejected the same way
// whichever door it comes through.
//
// Required-ness is deliberately NOT enforced here — that is completeness scoring's
// job (an absent value is "incomplete", not "invalid"). This package only judges
// values that are actually present.
package validation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// Rule is the per-attribute constraint set, parsed from attributes.validation.
// Every field is optional; a nil field means "no constraint".
type Rule struct {
	Pattern   *string  `json:"pattern,omitempty"`    // regex the (text) value must fully satisfy
	MinLength *int     `json:"min_length,omitempty"` // text
	MaxLength *int     `json:"max_length,omitempty"` // text
	Min       *float64 `json:"min,omitempty"`        // number/price
	Max       *float64 `json:"max,omitempty"`        // number/price
	MinSelect *int     `json:"min_select,omitempty"` // multiselect
	MaxSelect *int     `json:"max_select,omitempty"` // multiselect
}

// AttrDef is the validatable shape of an attribute definition.
type AttrDef struct {
	Code     string
	DataType string
	Options  []string
	Rule     Rule
}

// Violation is one failed constraint on one attribute (Code is the attribute code).
type Violation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ParseRule decodes the validation JSONB into a Rule (zero value on nil/invalid).
func ParseRule(raw []byte) Rule {
	var r Rule
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &r)
	}
	return r
}

// ParseOptions decodes the options JSONB (an array of strings) for select types.
func ParseOptions(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var opts []string
	if json.Unmarshal(raw, &opts) == nil {
		return opts
	}
	return nil
}

// ValidateAttributes checks a product's attributes JSON against the supplied
// attribute definitions (keyed by code). Only keys that match a definition are
// checked; unknown keys are ignored (they may be legacy or ad-hoc data).
func ValidateAttributes(defs map[string]AttrDef, attrs json.RawMessage) []Violation {
	if len(attrs) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(attrs, &m); err != nil {
		return []Violation{{Message: "attributes is not a JSON object"}}
	}
	var out []Violation
	for code, val := range m {
		def, ok := defs[code]
		if !ok {
			continue
		}
		for _, msg := range validateValue(def, val) {
			out = append(out, Violation{Code: code, Message: msg})
		}
	}
	return out
}

func validateValue(def AttrDef, val any) []string {
	if val == nil {
		return nil // absent value — completeness's concern, not validity
	}
	switch def.DataType {
	case "text", "file", "date":
		s, ok := val.(string)
		if !ok {
			return []string{def.Code + " must be text"}
		}
		return validateText(def, s)
	case "select":
		s, ok := val.(string)
		if !ok {
			return []string{def.Code + " must be a single selected value"}
		}
		if s != "" && len(def.Options) > 0 && !contains(def.Options, s) {
			return []string{def.Code + " must be one of the allowed options"}
		}
		return nil
	case "multiselect":
		arr, ok := val.([]any)
		if !ok {
			return []string{def.Code + " must be a list of values"}
		}
		return validateMultiselect(def, arr)
	case "number", "price":
		f, ok := toFloat(val)
		if !ok {
			return []string{def.Code + " must be a number"}
		}
		return validateNumber(def, f)
	case "boolean":
		if _, ok := val.(bool); !ok {
			return []string{def.Code + " must be true or false"}
		}
	}
	return nil
}

func validateText(def AttrDef, s string) []string {
	var out []string
	n := len([]rune(s))
	if def.Rule.MinLength != nil && n < *def.Rule.MinLength {
		out = append(out, fmt.Sprintf("%s must be at least %d characters", def.Code, *def.Rule.MinLength))
	}
	if def.Rule.MaxLength != nil && n > *def.Rule.MaxLength {
		out = append(out, fmt.Sprintf("%s must be at most %d characters", def.Code, *def.Rule.MaxLength))
	}
	if def.Rule.Pattern != nil && *def.Rule.Pattern != "" && s != "" {
		// A rule with an un-compilable pattern is a configuration mistake, not a
		// data error — skip it rather than block every write.
		if re, err := regexp.Compile(*def.Rule.Pattern); err == nil && !re.MatchString(s) {
			out = append(out, def.Code+" does not match the required format")
		}
	}
	return out
}

func validateNumber(def AttrDef, f float64) []string {
	var out []string
	if def.Rule.Min != nil && f < *def.Rule.Min {
		out = append(out, fmt.Sprintf("%s must be at least %s", def.Code, trimNum(*def.Rule.Min)))
	}
	if def.Rule.Max != nil && f > *def.Rule.Max {
		out = append(out, fmt.Sprintf("%s must be at most %s", def.Code, trimNum(*def.Rule.Max)))
	}
	return out
}

func validateMultiselect(def AttrDef, arr []any) []string {
	var out []string
	if def.Rule.MinSelect != nil && len(arr) < *def.Rule.MinSelect {
		out = append(out, fmt.Sprintf("%s needs at least %d selections", def.Code, *def.Rule.MinSelect))
	}
	if def.Rule.MaxSelect != nil && len(arr) > *def.Rule.MaxSelect {
		out = append(out, fmt.Sprintf("%s allows at most %d selections", def.Code, *def.Rule.MaxSelect))
	}
	if len(def.Options) > 0 {
		for _, item := range arr {
			s, ok := item.(string)
			if !ok || !contains(def.Options, s) {
				out = append(out, def.Code+" contains a value that is not an allowed option")
				break
			}
		}
	}
	return out
}

func contains(opts []string, s string) bool {
	for _, o := range opts {
		if o == s {
			return true
		}
	}
	return false
}

// toFloat accepts a JSON number or a numeric string (CSV import carries strings).
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func trimNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
