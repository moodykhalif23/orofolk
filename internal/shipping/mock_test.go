package shipping

import (
	"context"
	"testing"
)

func TestMockCarrierRatesApplySurcharge(t *testing.T) {
	c := MockCarrier{Name: "mock_fedex", Surcharge: "5.0000"}
	free := "100.0000"
	rows := []RateRow{
		{Service: "ground", Carrier: "", Amount: "10.0000"},
		{Service: "express", Carrier: "", Amount: "20.0000", FreeOver: &free},
	}
	// Subtotal below free-over: both priced (with surcharge).
	q, err := c.Rates(context.Background(), rows, RateRequest{Country: "US", Subtotal: "50.0000"})
	if err != nil {
		t.Fatalf("rates: %v", err)
	}
	if q[0].Amount != "15.0000" || q[0].Carrier != "mock_fedex" {
		t.Errorf("ground: want 15.0000/mock_fedex, got %s/%s", q[0].Amount, q[0].Carrier)
	}
	if q[1].Amount != "25.0000" || q[1].Free {
		t.Errorf("express below free-over: want 25.0000 not free, got %s free=%v", q[1].Amount, q[1].Free)
	}
	// Subtotal at/above free-over: express waived.
	q2, _ := c.Rates(context.Background(), rows, RateRequest{Country: "US", Subtotal: "100.0000"})
	if !q2[1].Free || q2[1].Amount != "0.0000" {
		t.Errorf("express free-over: want free 0.0000, got %s free=%v", q2[1].Amount, q2[1].Free)
	}
}

func TestMockCarrierLabelAndTrack(t *testing.T) {
	c := MockCarrier{}
	lbl, _ := c.CreateLabel(context.Background(), LabelRequest{ShipmentRef: "42", Service: "express"})
	if lbl.TrackingNumber != "MOCK-42" || lbl.Carrier != "mock_carrier" {
		t.Errorf("label: got %+v", lbl)
	}
	// Even last digit -> delivered; odd -> in_transit.
	if st, _ := c.Track(context.Background(), "MOCK-42"); st.Status != "delivered" {
		t.Errorf("track MOCK-42: want delivered, got %s", st.Status)
	}
	if st, _ := c.Track(context.Background(), "MOCK-43"); st.Status != "in_transit" {
		t.Errorf("track MOCK-43: want in_transit, got %s", st.Status)
	}
	if _, err := c.Track(context.Background(), ""); err == nil {
		t.Error("empty tracking should error")
	}
	if c.Provider() != "mock_carrier" {
		t.Errorf("provider: %s", c.Provider())
	}
}
