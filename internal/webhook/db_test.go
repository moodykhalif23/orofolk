package webhook_test

import (
	"context"
	"testing"

	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
)

const demoOrg = 1

// TestListActiveWebhookEndpointsForEvent verifies the fan-out query the worker
// uses: an empty event_types subscribes to everything, a populated one matches
// only its events, and inactive endpoints never match.
func TestListActiveWebhookEndpointsForEvent(t *testing.T) {
	pool := testsupport.NewDB(t)
	q := gen.New(pool)
	ctx := context.Background()

	all, err := q.CreateWebhookEndpoint(ctx, gen.CreateWebhookEndpointParams{
		OrganizationID: demoOrg, Url: "https://e/all", Secret: "s", EventTypes: []string{}, IsActive: true,
	})
	if err != nil {
		t.Fatalf("create all-events endpoint: %v", err)
	}
	specific, err := q.CreateWebhookEndpoint(ctx, gen.CreateWebhookEndpointParams{
		OrganizationID: demoOrg, Url: "https://e/orders", Secret: "s", EventTypes: []string{"order.placed"}, IsActive: true,
	})
	if err != nil {
		t.Fatalf("create specific endpoint: %v", err)
	}
	if _, err := q.CreateWebhookEndpoint(ctx, gen.CreateWebhookEndpointParams{
		OrganizationID: demoOrg, Url: "https://e/off", Secret: "s", EventTypes: []string{}, IsActive: false,
	}); err != nil {
		t.Fatalf("create inactive endpoint: %v", err)
	}

	// order.placed → the all-events endpoint AND the specific one (not the inactive).
	got := idSet(t, q, ctx, "order.placed")
	if !got[all.ID] || !got[specific.ID] {
		t.Errorf("order.placed matched %v, want both %d and %d", keys(got), all.ID, specific.ID)
	}

	// quote.created → only the all-events endpoint.
	got = idSet(t, q, ctx, "quote.created")
	if !got[all.ID] || got[specific.ID] {
		t.Errorf("quote.created matched %v, want only %d", keys(got), all.ID)
	}
}

// TestWebhookDeliveryLog confirms a delivery attempt is recorded and listed.
func TestWebhookDeliveryLog(t *testing.T) {
	pool := testsupport.NewDB(t)
	q := gen.New(pool)
	ctx := context.Background()

	ep, err := q.CreateWebhookEndpoint(ctx, gen.CreateWebhookEndpointParams{
		OrganizationID: demoOrg, Url: "https://e/log", Secret: "s", EventTypes: []string{}, IsActive: true,
	})
	if err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	if _, err := q.CreateWebhookDelivery(ctx, gen.CreateWebhookDeliveryParams{
		OrganizationID: demoOrg, EndpointID: ep.ID, EventType: "order.placed", Payload: []byte(`{"id":1}`),
		Status: "success", Attempt: 1, ResponseStatus: 200, Error: "",
	}); err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	rows, err := q.ListWebhookDeliveries(ctx, gen.ListWebhookDeliveriesParams{OrganizationID: demoOrg, EndpointID: ep.ID})
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "success" || rows[0].EventType != "order.placed" {
		t.Errorf("unexpected delivery rows: %+v", rows)
	}
}

func idSet(t *testing.T, q *gen.Queries, ctx context.Context, event string) map[int64]bool {
	t.Helper()
	eps, err := q.ListActiveWebhookEndpointsForEvent(ctx, gen.ListActiveWebhookEndpointsForEventParams{
		OrganizationID: demoOrg, Event: event,
	})
	if err != nil {
		t.Fatalf("ListActiveWebhookEndpointsForEvent(%q): %v", event, err)
	}
	out := make(map[int64]bool, len(eps))
	for _, e := range eps {
		out[e.ID] = true
	}
	return out
}

func keys(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
