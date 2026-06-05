package pricing

import "testing"

func sp(s string) *string { return &s }
func ip(i int64) *int64   { return &i }

func TestApplyPercentAndAmount(t *testing.T) {
	pct := []Rule{{AdjustmentType: "percent", AdjustmentValue: "-10"}}
	if got, _ := Apply("100.0000", nil, nil, pct); got != "90.0000" {
		t.Errorf("percent -10%%: want 90.0000, got %s", got)
	}
	amt := []Rule{{AdjustmentType: "amount", AdjustmentValue: "-2.5000"}}
	if got, _ := Apply("100.0000", nil, nil, amt); got != "97.5000" {
		t.Errorf("amount -2.50: want 97.5000, got %s", got)
	}
}

func TestApplyClampsAtZero(t *testing.T) {
	if got, _ := Apply("5.0000", nil, nil, []Rule{{AdjustmentType: "amount", AdjustmentValue: "-9"}}); got != "0.0000" {
		t.Errorf("clamp: want 0.0000, got %s", got)
	}
}

func TestApplyMatchesGroupAndAttribute(t *testing.T) {
	rules := []Rule{
		{CustomerGroupID: ip(7), AdjustmentType: "percent", AdjustmentValue: "-20"},
		{AttributeKey: sp("brand"), AttributeValue: sp("acme"), AdjustmentType: "percent", AdjustmentValue: "-5"},
	}
	// Wrong group, wrong attr -> no match -> unchanged.
	if got, _ := Apply("100.0000", map[string]string{"brand": "other"}, ip(9), rules); got != "100.0000" {
		t.Errorf("no match: want 100.0000, got %s", got)
	}
	// Matching group (first rule wins by order).
	if got, _ := Apply("100.0000", nil, ip(7), rules); got != "80.0000" {
		t.Errorf("group match: want 80.0000, got %s", got)
	}
	// No group match but attribute matches the 2nd rule.
	if got, _ := Apply("100.0000", map[string]string{"brand": "acme"}, ip(9), rules); got != "95.0000" {
		t.Errorf("attr match: want 95.0000, got %s", got)
	}
}

func TestApplyFirstRuleWins(t *testing.T) {
	// Rules are pre-sorted by priority desc; the first match applies.
	rules := []Rule{
		{AdjustmentType: "percent", AdjustmentValue: "-30", Priority: 10},
		{AdjustmentType: "percent", AdjustmentValue: "-5", Priority: 1},
	}
	if got, _ := Apply("100.0000", nil, nil, rules); got != "70.0000" {
		t.Errorf("priority: want 70.0000 (first match), got %s", got)
	}
}

func TestApplyLeavesNonNumericBase(t *testing.T) {
	if got, _ := Apply("", nil, nil, []Rule{{AdjustmentType: "percent", AdjustmentValue: "-10"}}); got != "" {
		t.Errorf("non-numeric base should be untouched, got %q", got)
	}
}
