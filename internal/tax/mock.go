package tax

import (
	"context"

	"b2bcommerce/internal/money"
)

// MockExternal is a deterministic stand-in for an external tax service
// (Avalara/TaxJar). It implements the same Adapter contract as LocalVAT, so it
// can be selected by config (NewServiceWithProvider) and a real service adapter
// later drops into the same seam. It applies a flat rate per tax class from its
// own table — simulating "the external service returned these rates" — rather
// than reading our DB rate table.
type MockExternal struct {
	// Rates maps tax_class -> fractional rate (e.g. "0.2000"). An unlisted class
	// falls back to Default.
	Rates   map[string]string
	Default string
}

// NewMockExternal builds a mock external provider with a default standard rate.
func NewMockExternal(defaultRate string, rates map[string]string) MockExternal {
	if rates == nil {
		rates = map[string]string{}
	}
	return MockExternal{Rates: rates, Default: defaultRate}
}

func (MockExternal) Provider() string { return "mock_external" }

func (m MockExternal) Calculate(_ context.Context, r Request) (Result, error) {
	res := Result{Country: r.Country}
	totals := make([]string, 0, len(r.Lines))
	for _, ln := range r.Lines {
		rate, ok := m.Rates[ln.TaxClass]
		if !ok {
			rate = m.Default
		}
		if rate == "" {
			rate = "0"
		}
		amt, err := taxOf(ln.Amount, rate)
		if err != nil {
			return Result{}, err
		}
		res.Lines = append(res.Lines, LineTax{ProductID: ln.ProductID, TaxClass: ln.TaxClass, Rate: rate, TaxAmount: amt})
		totals = append(totals, amt)
	}
	total, err := money.Sum(totals...)
	if err != nil {
		return Result{}, err
	}
	if total == "" {
		total = "0"
	}
	res.TaxTotal = total
	return res, nil
}
