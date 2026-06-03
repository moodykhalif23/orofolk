// Package tax implements the tax adapter (Pack 2 §4.4). The built-in provider is
// a rules-based local VAT calculator: per-line tax = line amount × rate, where
// the rate is resolved from (country, tax_class) — no external calls. An
// external service (Avalara/TaxJar-style) can implement the same Adapter later.
package tax

import (
	"context"

	"b2bcommerce/internal/money"
)

// Line is one taxable line: its product's tax class and the amount to tax
// (typically the line's row_total).
type Line struct {
	ProductID int64  `json:"product_id"`
	TaxClass  string `json:"tax_class"`
	Amount    string `json:"amount"`
}

// LineTax is the computed tax for one line.
type LineTax struct {
	ProductID int64  `json:"product_id"`
	TaxClass  string `json:"tax_class"`
	Rate      string `json:"rate"`
	TaxAmount string `json:"tax_amount"`
}

// Request is a tax calculation for a destination country + lines.
type Request struct {
	Country string `json:"country"`
	Lines   []Line `json:"lines"`
}

// Result is per-line tax plus the order-level total.
type Result struct {
	Country  string    `json:"country"`
	Lines    []LineTax `json:"lines"`
	TaxTotal string    `json:"tax_total"`
}

// Adapter is the tax-provider contract (§4.4).
type Adapter interface {
	Provider() string
	Calculate(ctx context.Context, r Request) (Result, error)
}

// LocalVAT computes VAT from a rate table keyed by (country, tax_class). It is
// constructed per-request from the rates loaded for the destination country.
type LocalVAT struct {
	// rates maps tax_class -> fractional rate string (e.g. "0.1600"). A class
	// with no entry (or no rates at all) is untaxed.
	rates map[string]string
}

// NewLocalVAT builds the provider from a country's class→rate map.
func NewLocalVAT(rates map[string]string) LocalVAT { return LocalVAT{rates: rates} }

func (LocalVAT) Provider() string { return "local_vat" }

func (p LocalVAT) Calculate(_ context.Context, r Request) (Result, error) {
	res := Result{Country: r.Country}
	totals := make([]string, 0, len(r.Lines))
	for _, ln := range r.Lines {
		rate := p.rates[ln.TaxClass]
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

// taxOf returns amount × rate, money-rounded.
func taxOf(amount, rate string) (string, error) {
	a, err := money.Parse(amount)
	if err != nil {
		return "", err
	}
	rt, err := money.Parse(rate)
	if err != nil {
		return "", err
	}
	a.Mul(a, rt)
	return money.Format(a), nil
}
