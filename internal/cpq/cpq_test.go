package cpq

import "testing"

func def() Definition {
	return Definition{
		ProductID: 1, BasePrice: "100.0000", Currency: "USD",
		Groups: []Group{
			{ID: 1, Code: "color", Name: "Color", Required: true, MinSelect: 1, MaxSelect: 1, Options: []Option{
				{ID: 1, GroupID: 1, Code: "red", PriceDelta: "0"},
				{ID: 2, GroupID: 1, Code: "blue", PriceDelta: "10.0000"},
			}},
			{ID: 2, Code: "warranty", Name: "Warranty", Required: false, MaxSelect: 1, Options: []Option{
				{ID: 3, GroupID: 2, Code: "ext", PriceDelta: "25.0000"},
			}},
		},
	}
}

func TestConfigurePricesValid(t *testing.T) {
	r := Configure(def(), []int64{2, 3}) // blue + extended warranty
	if !r.Valid {
		t.Fatalf("expected valid, errors=%v", r.Errors)
	}
	if r.UnitPrice != "135.0000" { // 100 + 10 + 25
		t.Errorf("unit price = %s, want 135.0000", r.UnitPrice)
	}
	if len(r.Selected) != 2 {
		t.Errorf("selected = %d, want 2", len(r.Selected))
	}
}

func TestConfigureRequiredGroup(t *testing.T) {
	r := Configure(def(), nil) // no color
	if r.Valid {
		t.Fatal("expected invalid (color required)")
	}
}

func TestConfigureMaxSelect(t *testing.T) {
	r := Configure(def(), []int64{1, 2}) // two colors, max 1
	if r.Valid {
		t.Fatal("expected invalid (max 1 color)")
	}
}

func TestConfigureUnknownOption(t *testing.T) {
	if r := Configure(def(), []int64{1, 999}); r.Valid {
		t.Fatal("expected invalid (unknown option)")
	}
}

func TestConfigureRequiresRule(t *testing.T) {
	d := def()
	d.Rules = []Rule{{Kind: "requires", OptionID: 3, RelatedOptionID: 2}} // ext requires blue
	if r := Configure(d, []int64{1, 3}); r.Valid {                        // red + ext, but ext needs blue
		t.Fatal("expected invalid (ext requires blue)")
	}
	if r := Configure(d, []int64{2, 3}); !r.Valid { // blue + ext satisfies
		t.Fatalf("expected valid, errors=%v", r.Errors)
	}
}

func TestConfigureExcludesRule(t *testing.T) {
	d := def()
	d.Rules = []Rule{{Kind: "excludes", OptionID: 1, RelatedOptionID: 3}} // red excludes ext
	if r := Configure(d, []int64{1, 3}); r.Valid {
		t.Fatal("expected invalid (red excludes ext)")
	}
	if r := Configure(d, []int64{2, 3}); !r.Valid {
		t.Fatalf("expected valid, errors=%v", r.Errors)
	}
}
