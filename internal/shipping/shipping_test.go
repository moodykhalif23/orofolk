package shipping

import (
	"context"
	"testing"
)

func ptr(s string) *string { return &s }

func TestLocalRatesFreeOver(t *testing.T) {
	lp := Local{}
	rows := []RateRow{
		{Service: "standard", Carrier: "local", Amount: "10.0000"},
		{Service: "express", Carrier: "local", Amount: "25.0000", FreeOver: ptr("100.0000")},
	}
	q, err := lp.Rates(context.Background(), rows, RateRequest{Country: "KE", Subtotal: "150.0000"})
	if err != nil {
		t.Fatalf("rates: %v", err)
	}
	if len(q) != 2 {
		t.Fatalf("quotes = %d, want 2", len(q))
	}
	if q[0].Amount != "10.0000" || q[0].Free {
		t.Errorf("standard = %+v", q[0])
	}
	if !q[1].Free || q[1].Amount != "0.0000" { // subtotal 150 >= free_over 100
		t.Errorf("express should be free over threshold: %+v", q[1])
	}
}

func TestLocalRatesBelowThreshold(t *testing.T) {
	lp := Local{}
	rows := []RateRow{{Service: "express", Carrier: "local", Amount: "25.0000", FreeOver: ptr("100.0000")}}
	q, _ := lp.Rates(context.Background(), rows, RateRequest{Country: "KE", Subtotal: "50.0000"})
	if q[0].Free || q[0].Amount != "25.0000" {
		t.Errorf("below threshold should charge: %+v", q[0])
	}
}

func TestLocalLabelAndTrack(t *testing.T) {
	lp := Local{}
	l, err := lp.CreateLabel(context.Background(), LabelRequest{ShipmentRef: "abc-123", Service: "express"})
	if err != nil {
		t.Fatalf("label: %v", err)
	}
	if l.TrackingNumber != "LOCAL-abc-123" {
		t.Errorf("tracking = %q", l.TrackingNumber)
	}
	// Deterministic (idempotent retry).
	l2, _ := lp.CreateLabel(context.Background(), LabelRequest{ShipmentRef: "abc-123"})
	if l2.TrackingNumber != l.TrackingNumber {
		t.Error("label creation should be deterministic")
	}
	st, err := lp.Track(context.Background(), l.TrackingNumber)
	if err != nil || st.Status != "in_transit" {
		t.Errorf("track = %+v err=%v", st, err)
	}
	if _, err := lp.Track(context.Background(), ""); err == nil {
		t.Error("empty tracking should error")
	}
}
