// Package shipping implements the shipping adapter (Pack 2 §4.3). The built-in
// provider is a rules-based table-rate calculator (config-driven rates by
// country + service, with free-over-threshold), plus deterministic label
// creation and tracking suitable for self-hosting. A real carrier (e.g.
// EasyPost/Shippo) can implement the same Adapter behind a connection row.
package shipping

import (
	"context"
	"fmt"

	"b2bcommerce/internal/money"
)

// RateRow is a configured table-rate (loaded from shipping_rates).
type RateRow struct {
	Service  string
	Carrier  string
	Amount   string
	FreeOver *string // waive the fee at/above this subtotal; nil = never free
}

// RateRequest asks for quotes to a destination, given the order subtotal.
type RateRequest struct {
	Country  string `json:"country"`
	Subtotal string `json:"subtotal"`
}

// RateQuote is one offered shipping option.
type RateQuote struct {
	Service string `json:"service"`
	Carrier string `json:"carrier"`
	Amount  string `json:"amount"`
	Free    bool   `json:"free"`
}

// LabelRequest creates a shipping label for a shipment.
type LabelRequest struct {
	ShipmentRef string `json:"shipment_ref"`
	Service     string `json:"service"`
	Country     string `json:"country"`
}

// Label is the created-label result.
type Label struct {
	Carrier        string `json:"carrier"`
	Service        string `json:"service"`
	TrackingNumber string `json:"tracking_number"`
	LabelURL       string `json:"label_url"`
}

// TrackingStatus is the current state of a shipment in transit.
type TrackingStatus struct {
	TrackingNumber string `json:"tracking_number"`
	Status         string `json:"status"`
	Carrier        string `json:"carrier"`
}

// Adapter is the shipping-provider contract (§4.3).
type Adapter interface {
	Provider() string
	Rates(ctx context.Context, rows []RateRow, r RateRequest) ([]RateQuote, error)
	CreateLabel(ctx context.Context, r LabelRequest) (Label, error)
	Track(ctx context.Context, tracking string) (TrackingStatus, error)
}

// Local is the table-rate provider.
type Local struct{}

func (Local) Provider() string { return "local" }

// Rates turns configured rows into quotes, applying the free-over threshold.
func (Local) Rates(_ context.Context, rows []RateRow, r RateRequest) ([]RateQuote, error) {
	quotes := make([]RateQuote, 0, len(rows))
	for _, row := range rows {
		amount, free := row.Amount, false
		if row.FreeOver != nil {
			if cmp, err := money.Cmp(r.Subtotal, *row.FreeOver); err == nil && cmp >= 0 {
				amount, free = "0.0000", true
			}
		}
		quotes = append(quotes, RateQuote{Service: row.Service, Carrier: defCarrier(row.Carrier), Amount: amount, Free: free})
	}
	return quotes, nil
}

// CreateLabel generates a deterministic tracking number from the shipment ref
// (so retries are idempotent) — a real carrier would call its label API.
func (Local) CreateLabel(_ context.Context, r LabelRequest) (Label, error) {
	tracking := "LOCAL-" + r.ShipmentRef
	return Label{
		Carrier: "local", Service: def(r.Service, "standard"),
		TrackingNumber: tracking, LabelURL: "/files/labels/" + tracking + ".pdf",
	}, nil
}

// Track returns a status for a tracking number. The local provider reports
// "in_transit" for any issued label (carriers would query their API).
func (Local) Track(_ context.Context, tracking string) (TrackingStatus, error) {
	if tracking == "" {
		return TrackingStatus{}, fmt.Errorf("empty tracking number")
	}
	return TrackingStatus{TrackingNumber: tracking, Status: "in_transit", Carrier: "local"}, nil
}

func def(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
func defCarrier(c string) string { return def(c, "local") }
