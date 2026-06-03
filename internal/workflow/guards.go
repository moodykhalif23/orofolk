package workflow

import (
	"context"
	"fmt"
	"strconv"
)

// AmountLteLimit is the built-in `amount_lte_limit` guard (Pack 2 §3.6 order
// approval): it allows a transition only when a numeric amount is within a
// limit. Both are read from the transition Data by configurable keys:
//
//	params.field       — payload key holding the amount   (default "grand_total")
//	params.limit_field — payload key holding the limit     (default "spending_limit")
//
// A missing or non-positive limit means "no limit configured" → allowed, so the
// guard is inert until a real limit is present (e.g. a buyer's spending_limit).
type AmountLteLimit struct{}

func (AmountLteLimit) Key() string { return "amount_lte_limit" }

func (AmountLteLimit) Allow(_ context.Context, in GuardInput) (bool, string, error) {
	field := paramString(in.Params, "field", "grand_total")
	limitField := paramString(in.Params, "limit_field", "spending_limit")

	limit, ok := toNumber(in.Data[limitField])
	if !ok || limit <= 0 {
		return true, "", nil // no approval limit set → allow
	}
	amount, ok := toNumber(in.Data[field])
	if !ok {
		return true, "", nil // nothing to compare → allow
	}
	if amount <= limit {
		return true, "", nil
	}
	return false, fmt.Sprintf("amount %.2f exceeds the approval limit of %.2f", amount, limit), nil
}

// HasPermission is the built-in `has_permission` guard: it allows a transition
// only when the actor holds a required permission (params.permission). The
// caller supplies the actor's permissions in the transition Data under
// "actor_permissions" ([]string). Used to gate order approval (resume out of
// on_hold requires `order.approve`).
type HasPermission struct{}

func (HasPermission) Key() string { return "has_permission" }

func (HasPermission) Allow(_ context.Context, in GuardInput) (bool, string, error) {
	required := paramString(in.Params, "permission", "")
	if required == "" {
		return true, "", nil
	}
	for _, p := range actorPermissions(in.Data["actor_permissions"]) {
		if p == required {
			return true, "", nil
		}
	}
	return false, "requires the " + required + " permission", nil
}

// actorPermissions normalizes the permission list, which may arrive as []string
// (built in Go) or []any (round-tripped through JSON).
func actorPermissions(v any) []string {
	switch list := v.(type) {
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, p := range list {
			if s, ok := p.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func paramString(m map[string]any, key, def string) string {
	if m != nil {
		if s, ok := m[key].(string); ok && s != "" {
			return s
		}
	}
	return def
}

// toNumber coerces money strings / JSON numbers to float64.
func toNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}
