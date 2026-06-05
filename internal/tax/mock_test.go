package tax

import (
	"context"
	"testing"

	"b2bcommerce/internal/store/gen"
)

func TestMockExternalCalculate(t *testing.T) {
	p := NewMockExternal("0.2000", map[string]string{"zero": "0"})
	res, err := p.Calculate(context.Background(), Request{
		Country: "US",
		Lines: []Line{
			{ProductID: 1, TaxClass: "standard", Amount: "100.0000"}, // default 20%
			{ProductID: 2, TaxClass: "zero", Amount: "50.0000"},      // explicit 0
		},
	})
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if res.Lines[0].TaxAmount != "20.0000" || res.Lines[1].TaxAmount != "0.0000" {
		t.Fatalf("mock external rates: got %+v", res.Lines)
	}
	if res.TaxTotal != "20.0000" {
		t.Errorf("total: want 20.0000, got %s", res.TaxTotal)
	}
	if p.Provider() != "mock_external" {
		t.Errorf("provider tag: %s", p.Provider())
	}
}

// fakeQuerier returns product tax classes but NO DB rate rows — proving the
// external provider path doesn't depend on the local rate table.
type fakeQuerier struct{ class string }

func (f fakeQuerier) ListTaxRatesByCountry(context.Context, gen.ListTaxRatesByCountryParams) ([]gen.ListTaxRatesByCountryRow, error) {
	return nil, nil
}
func (f fakeQuerier) GetProductTaxClasses(_ context.Context, arg gen.GetProductTaxClassesParams) ([]gen.GetProductTaxClassesRow, error) {
	out := make([]gen.GetProductTaxClassesRow, 0, len(arg.Column2))
	for _, id := range arg.Column2 {
		out = append(out, gen.GetProductTaxClassesRow{ID: id, TaxClass: f.class})
	}
	return out, nil
}

func TestServiceUsesExternalProvider(t *testing.T) {
	// No DB rates configured; the external provider still taxes at 20%.
	svc := NewServiceWithProvider(fakeQuerier{class: "standard"}, NewMockExternal("0.2000", nil))
	perLine, total, err := svc.ComputeOrderTax(context.Background(), 1, "US", []OrderLine{
		{ProductID: 1, Amount: "100.0000"},
		{ProductID: 2, Amount: "10.0000"},
	})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if perLine[0] != "20.0000" || perLine[1] != "2.0000" || total != "22.0000" {
		t.Errorf("external provider order tax: perLine=%v total=%s", perLine, total)
	}
}

func TestServiceDefaultLocalNoRatesUntaxed(t *testing.T) {
	// No provider + no DB rates -> untaxed (unchanged default behaviour).
	svc := NewService(fakeQuerier{class: "standard"})
	_, total, err := svc.ComputeOrderTax(context.Background(), 1, "US", []OrderLine{{ProductID: 1, Amount: "100.0000"}})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if total != "0" {
		t.Errorf("no rates: want 0, got %s", total)
	}
}
