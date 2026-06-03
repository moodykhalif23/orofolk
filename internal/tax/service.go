package tax

import (
	"context"

	"b2bcommerce/internal/store/gen"
)

// Querier is the DB surface the Service needs.
type Querier interface {
	ListTaxRatesByCountry(ctx context.Context, arg gen.ListTaxRatesByCountryParams) ([]gen.ListTaxRatesByCountryRow, error)
	GetProductTaxClasses(ctx context.Context, arg gen.GetProductTaxClassesParams) ([]gen.GetProductTaxClassesRow, error)
}

// OrderLine is a taxable order line: a product and its line amount (row_total).
type OrderLine struct {
	ProductID int64
	Amount    string
}

// Service resolves rates + product tax classes from the DB and computes order
// tax via the LocalVAT provider.
type Service struct{ q Querier }

func NewService(q Querier) *Service { return &Service{q: q} }

// ComputeOrderTax returns per-line tax (aligned to lines) and the total for a
// destination country. With no country or no configured rates it returns all
// zeros — so untaxed orgs/regions are unaffected.
func (s *Service) ComputeOrderTax(ctx context.Context, org int64, country string, lines []OrderLine) (perLine []string, total string, err error) {
	perLine = make([]string, len(lines))
	for i := range perLine {
		perLine[i] = "0"
	}
	if country == "" || len(lines) == 0 {
		return perLine, "0", nil
	}
	rateRows, err := s.q.ListTaxRatesByCountry(ctx, gen.ListTaxRatesByCountryParams{OrganizationID: org, Country: country})
	if err != nil {
		return perLine, "0", err
	}
	if len(rateRows) == 0 {
		return perLine, "0", nil
	}
	rateMap := make(map[string]string, len(rateRows))
	for _, rr := range rateRows {
		rateMap[rr.TaxClass] = rr.Rate
	}

	ids := make([]int64, 0, len(lines))
	for _, ln := range lines {
		ids = append(ids, ln.ProductID)
	}
	classRows, err := s.q.GetProductTaxClasses(ctx, gen.GetProductTaxClassesParams{OrganizationID: org, Column2: ids})
	if err != nil {
		return perLine, "0", err
	}
	classByID := make(map[int64]string, len(classRows))
	for _, cr := range classRows {
		classByID[cr.ID] = cr.TaxClass
	}

	req := Request{Country: country, Lines: make([]Line, len(lines))}
	for i, ln := range lines {
		cls := classByID[ln.ProductID]
		if cls == "" {
			cls = "standard"
		}
		req.Lines[i] = Line{ProductID: ln.ProductID, TaxClass: cls, Amount: ln.Amount}
	}
	res, err := NewLocalVAT(rateMap).Calculate(ctx, req)
	if err != nil {
		return perLine, "0", err
	}
	for i := range res.Lines {
		perLine[i] = res.Lines[i].TaxAmount
	}
	return perLine, res.TaxTotal, nil
}
