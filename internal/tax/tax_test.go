package tax

import (
	"context"
	"testing"
)

func TestLocalVATCalculate(t *testing.T) {
	p := NewLocalVAT(map[string]string{"standard": "0.1600", "exempt": "0"})
	res, err := p.Calculate(context.Background(), Request{
		Country: "KE",
		Lines: []Line{
			{ProductID: 1, TaxClass: "standard", Amount: "100.0000"},
			{ProductID: 2, TaxClass: "exempt", Amount: "50.0000"},
			{ProductID: 3, TaxClass: "standard", Amount: "200.0000"},
		},
	})
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if res.Lines[0].TaxAmount != "16.0000" {
		t.Errorf("line1 tax = %s, want 16.0000", res.Lines[0].TaxAmount)
	}
	if res.Lines[1].TaxAmount != "0.0000" {
		t.Errorf("exempt line tax = %s, want 0.0000", res.Lines[1].TaxAmount)
	}
	if res.Lines[2].TaxAmount != "32.0000" {
		t.Errorf("line3 tax = %s, want 32.0000", res.Lines[2].TaxAmount)
	}
	if res.TaxTotal != "48.0000" { // 16 + 0 + 32
		t.Errorf("tax total = %s, want 48.0000", res.TaxTotal)
	}
}

func TestLocalVATUnknownClassUntaxed(t *testing.T) {
	p := NewLocalVAT(map[string]string{"standard": "0.16"})
	res, _ := p.Calculate(context.Background(), Request{Country: "KE", Lines: []Line{{ProductID: 1, TaxClass: "weird", Amount: "100"}}})
	if res.TaxTotal != "0.0000" {
		t.Errorf("unknown class should be untaxed, got %s", res.TaxTotal)
	}
}
