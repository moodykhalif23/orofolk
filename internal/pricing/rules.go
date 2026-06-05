// Package pricing holds the rule-based price-adjustment engine (PRD §7.2): a
// pure function that adjusts a resolved base price by the highest-priority
// matching rule. It is additive — with no matching rule the base is returned
// unchanged — and has no DB or clock dependency so it is trivially testable.
package pricing

import (
	"math/big"

	"b2bcommerce/internal/money"
)

// Rule is one price-adjustment rule. A nil CustomerGroupID matches all buyers;
// a nil AttributeKey matches all products. AdjustmentType is "percent" (value
// is a percentage, e.g. -10 for -10%) or "amount" (a signed currency delta).
type Rule struct {
	CustomerGroupID *int64
	AttributeKey    *string
	AttributeValue  *string
	AdjustmentType  string
	AdjustmentValue string
	Priority        int32
}

// Matches reports whether a rule applies to a product (its attribute map) and a
// buyer (their customer group, nil if none).
func (r Rule) Matches(attrs map[string]string, groupID *int64) bool {
	if r.CustomerGroupID != nil {
		if groupID == nil || *groupID != *r.CustomerGroupID {
			return false
		}
	}
	if r.AttributeKey != nil {
		if attrs[*r.AttributeKey] != deref(r.AttributeValue) {
			return false
		}
	}
	return true
}

// Apply returns the base price adjusted by the first matching rule. Rules must
// be pre-sorted by priority (desc) — ListActivePriceAdjustmentRules does this.
// The result is never negative. With no match (or an unparseable base) the base
// is returned unchanged.
func Apply(base string, attrs map[string]string, groupID *int64, rules []Rule) (string, error) {
	baseR, err := money.Parse(base)
	if err != nil {
		return base, nil // leave non-numeric base (e.g. price-on-request) untouched
	}
	for _, rule := range rules {
		if !rule.Matches(attrs, groupID) {
			continue
		}
		val, err := money.Parse(rule.AdjustmentValue)
		if err != nil {
			return "", err
		}
		var out *big.Rat
		switch rule.AdjustmentType {
		case "percent":
			// base * (1 + val/100)
			factor := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(val, big.NewRat(100, 1)))
			out = new(big.Rat).Mul(baseR, factor)
		case "amount":
			out = new(big.Rat).Add(baseR, val)
		default:
			return base, nil
		}
		if out.Sign() < 0 {
			out = new(big.Rat) // clamp at zero
		}
		return money.Format(out), nil
	}
	return money.Format(baseR), nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
