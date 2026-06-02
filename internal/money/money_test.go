package money_test

import (
	"testing"

	"b2bcommerce/internal/money"
)

func TestLineTotalAndSum(t *testing.T) {
	// Exact decimal arithmetic — no binary-float drift.
	got, err := money.LineTotal("3", "10.0000")
	if err != nil || got != "30.0000" {
		t.Fatalf("LineTotal 3*10: want 30.0000, got %q err=%v", got, err)
	}
	// 0.1 * 3 is famously lossy in float; must be exact here.
	got, err = money.LineTotal("3", "0.1000")
	if err != nil || got != "0.3000" {
		t.Fatalf("LineTotal 3*0.1: want 0.3000, got %q err=%v", got, err)
	}
	sum, err := money.Sum("20.0000", "0.3000", "9.7000")
	if err != nil || sum != "30.0000" {
		t.Fatalf("Sum: want 30.0000, got %q err=%v", sum, err)
	}
	if _, err := money.Parse("not-money"); err == nil {
		t.Fatal("Parse: expected error on invalid decimal")
	}
}
