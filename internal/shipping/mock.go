package shipping

import (
	"context"
	"fmt"

	"b2bcommerce/internal/money"
)

// MockCarrier is a deterministic stand-in for a real carrier API (FedEx/UPS/DHL).
// It implements the same Adapter contract as Local, so it can be selected by
// config (WithShippingProvider) without touching call sites — and a real carrier
// adapter later drops into the same seam. Quotes apply a fixed fuel surcharge so
// they're distinguishable from the local table-rate, but stay fully deterministic
// (no clock/network) for tests.
type MockCarrier struct {
	// Name is the carrier label surfaced on quotes/labels (e.g. "mock_fedex").
	Name string
	// Surcharge is added to each configured rate to mimic carrier fees.
	Surcharge string
}

func (m MockCarrier) carrier() string { return def(m.Name, "mock_carrier") }

func (MockCarrier) Provider() string { return "mock_carrier" }

// Rates returns the configured rows priced by the "carrier": each amount gets
// the surcharge added (free-over thresholds still waive the fee entirely).
func (m MockCarrier) Rates(_ context.Context, rows []RateRow, r RateRequest) ([]RateQuote, error) {
	surcharge := def(m.Surcharge, "0")
	quotes := make([]RateQuote, 0, len(rows))
	for _, row := range rows {
		if row.FreeOver != nil {
			if cmp, err := money.Cmp(r.Subtotal, *row.FreeOver); err == nil && cmp >= 0 {
				quotes = append(quotes, RateQuote{Service: row.Service, Carrier: m.carrier(), Amount: "0.0000", Free: true})
				continue
			}
		}
		amount, err := money.Sum(row.Amount, surcharge)
		if err != nil {
			return nil, err
		}
		quotes = append(quotes, RateQuote{Service: row.Service, Carrier: m.carrier(), Amount: amount, Free: false})
	}
	return quotes, nil
}

// CreateLabel mints a deterministic carrier tracking number from the shipment
// ref (idempotent across retries) — a real adapter would call the carrier's API.
func (m MockCarrier) CreateLabel(_ context.Context, r LabelRequest) (Label, error) {
	tracking := "MOCK-" + r.ShipmentRef
	return Label{
		Carrier: m.carrier(), Service: def(r.Service, "standard"),
		TrackingNumber: tracking, LabelURL: "/files/labels/" + tracking + ".pdf",
	}, nil
}

// Track derives a deterministic status from the tracking number so tests are
// stable: refs ending in an even digit read "delivered", else "in_transit".
func (m MockCarrier) Track(_ context.Context, tracking string) (TrackingStatus, error) {
	if tracking == "" {
		return TrackingStatus{}, fmt.Errorf("empty tracking number")
	}
	status := "in_transit"
	last := tracking[len(tracking)-1]
	if last >= '0' && last <= '9' && (last-'0')%2 == 0 {
		status = "delivered"
	}
	return TrackingStatus{TrackingNumber: tracking, Status: status, Carrier: m.carrier()}, nil
}
