package notify

import (
	"context"
	"fmt"
)

// FromEvent maps a committed domain event into in-app notifications. It is
// invoked by the worker after the event's automation rules run, so notification
// delivery is decoupled from the request that produced the event and a mapping
// gap never breaks anything upstream.
//
// Every payload must carry "org_id"; recipient scoping comes from the event's
// own ids (customer_id, vendor_id). Unknown events are ignored.
func (s *Service) FromEvent(ctx context.Context, event string, payload map[string]any) {
	orgID, ok := pint(payload, "org_id")
	if !ok {
		return // can't route without the tenant
	}

	switch event {
	case "order.placed":
		// New order → notify the operator's admins. If the order touched specific
		// vendors, notify each of those vendors too.
		num := pstr(payload, "order_number")
		s.NotifyAdmins(ctx, orgID, Template{
			Type:     event,
			Title:    "New order " + num,
			Body:     money(payload),
			Link:     adminOrderLink(payload),
			Severity: "success",
			Data:     payload,
		})
		for _, vid := range pint64s(payload, "vendor_ids") {
			s.NotifyVendorUsers(ctx, orgID, vid, Template{
				Type:     event,
				Title:    "New order " + num,
				Body:     "You have items to fulfil on this order.",
				Link:     "/orders",
				Severity: "success",
				Data:     payload,
			})
		}

	case "order.status_changed":
		cid, ok := pint(payload, "customer_id")
		if !ok {
			return
		}
		num := pstr(payload, "order_number")
		status := pstr(payload, "status")
		s.NotifyCustomerUsers(ctx, orgID, cid, Template{
			Type:     event,
			Title:    fmt.Sprintf("Order %s is now %s", num, status),
			Link:     storefrontOrderLink(payload),
			Severity: "info",
			Data:     payload,
		})

	case "quote.sent":
		cid, ok := pint(payload, "customer_id")
		if !ok {
			return
		}
		s.NotifyCustomerUsers(ctx, orgID, cid, Template{
			Type:     event,
			Title:    "A new quote is ready: " + pstr(payload, "quote_number"),
			Body:     "Review and accept it to place your order.",
			Link:     "/account/quotes",
			Severity: "info",
			Data:     payload,
		})

	case "rfq.submitted":
		s.NotifyAdmins(ctx, orgID, Template{
			Type:     event,
			Title:    "New request for quote " + pstr(payload, "reference"),
			Body:     "A buyer is awaiting pricing.",
			Link:     "/sales/rfqs",
			Severity: "info",
			Data:     payload,
		})

	case "return.requested":
		s.NotifyAdmins(ctx, orgID, Template{
			Type:     event,
			Title:    "Return requested " + pstr(payload, "rma_number"),
			Link:     "/sales/returns",
			Severity: "warning",
			Data:     payload,
		})

	case "approval.requested":
		cid, ok := pint(payload, "customer_id")
		if !ok {
			return
		}
		s.NotifyCustomerApprovers(ctx, orgID, cid, Template{
			Type:     event,
			Title:    "Order " + pstr(payload, "order_number") + " needs your approval",
			Link:     "/account/approvals",
			Severity: "warning",
			Data:     payload,
		})
	}
}

// --- payload helpers (JSON numbers arrive as float64) ---

func pstr(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func pint(m map[string]any, k string) (int64, bool) {
	switch v := m[k].(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}

func pint64s(m map[string]any, k string) []int64 {
	raw, ok := m[k].([]any)
	if !ok {
		return nil
	}
	out := make([]int64, 0, len(raw))
	for _, e := range raw {
		switch v := e.(type) {
		case float64:
			out = append(out, int64(v))
		case int64:
			out = append(out, v)
		}
	}
	return out
}

func money(m map[string]any) string {
	total := pstr(m, "grand_total")
	if total == "" {
		return ""
	}
	if cur := pstr(m, "currency"); cur != "" {
		return cur + " " + total
	}
	return total
}

func adminOrderLink(m map[string]any) string {
	if id := pstr(m, "order_public_id"); id != "" {
		return "/orders/" + id
	}
	return "/orders"
}

func storefrontOrderLink(m map[string]any) string {
	if id := pstr(m, "order_public_id"); id != "" {
		return "/account/orders/" + id
	}
	return "/account/orders"
}
