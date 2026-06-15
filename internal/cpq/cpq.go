// Package cpq is the Configure-Price-Quote engine (PRD §8): it validates a
// chosen configuration of a configurable product against its option groups and
// rules, and recomputes the line price (base + option deltas). Pure and
// dependency-free so it is exhaustively unit-testable; handlers load the
// Definition from the DB and feed selections in.
package cpq

import (
	"fmt"
	"sort"

	"b2bcommerce/internal/money"
)

// Option is one selectable choice within a group; PriceDelta adds to the base.
type Option struct {
	ID         int64  `json:"id"`
	GroupID    int64  `json:"group_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	PriceDelta string `json:"price_delta"`
	IsDefault  bool   `json:"is_default"`
}

// Group bundles mutually-related options with select constraints.
type Group struct {
	ID        int64    `json:"id"`
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Required  bool     `json:"required"`
	MinSelect int      `json:"min_select"`
	MaxSelect int      `json:"max_select"`
	Options   []Option `json:"options"`
}

// Rule is a pairwise constraint across options: "requires" (if A then B) or
// "excludes" (not both A and B).
type Rule struct {
	Kind            string `json:"kind"` // requires | excludes
	OptionID        int64  `json:"option_id"`
	RelatedOptionID int64  `json:"related_option_id"`
}

// Definition is a configurable product's full configuration model.
type Definition struct {
	ProductID int64   `json:"product_id"`
	BasePrice string  `json:"base_price"`
	Currency  string  `json:"currency"`
	Groups    []Group `json:"groups"`
	Rules     []Rule  `json:"rules"`
}

// SelectedOption is one resolved choice in a priced configuration.
type SelectedOption struct {
	OptionID   int64  `json:"option_id"`
	GroupCode  string `json:"group_code"`
	OptionCode string `json:"option_code"`
	Name       string `json:"name"`
	PriceDelta string `json:"price_delta"`
}

// Result is the outcome of configuring a product.
type Result struct {
	Valid     bool             `json:"valid"`
	Errors    []string         `json:"errors"`
	BasePrice string           `json:"base_price"`
	UnitPrice string           `json:"unit_price"`
	Currency  string           `json:"currency"`
	Selected  []SelectedOption `json:"selected"`
}

func Configure(def Definition, selectedIDs []int64) Result {
	res := Result{BasePrice: def.BasePrice, Currency: def.Currency}

	// Index options by id and selections into a set.
	optByID := map[int64]Option{}
	groupByID := map[int64]Group{}
	for _, g := range def.Groups {
		groupByID[g.ID] = g
		for _, o := range g.Options {
			optByID[o.ID] = o
		}
	}
	selected := map[int64]bool{}
	for _, id := range selectedIDs {
		if _, ok := optByID[id]; !ok {
			res.Errors = append(res.Errors, fmt.Sprintf("unknown option %d", id))
			continue
		}
		selected[id] = true
	}

	// Per-group select-count constraints.
	for _, g := range def.Groups {
		count := 0
		for _, o := range g.Options {
			if selected[o.ID] {
				count++
			}
		}
		min := g.MinSelect
		if g.Required && min < 1 {
			min = 1
		}
		if count < min {
			res.Errors = append(res.Errors, fmt.Sprintf("group %q requires at least %d selection(s)", g.Code, min))
		}
		if g.MaxSelect > 0 && count > g.MaxSelect {
			res.Errors = append(res.Errors, fmt.Sprintf("group %q allows at most %d selection(s)", g.Code, g.MaxSelect))
		}
	}

	// Pairwise rules.
	for _, r := range def.Rules {
		switch r.Kind {
		case "requires":
			if selected[r.OptionID] && !selected[r.RelatedOptionID] {
				res.Errors = append(res.Errors, fmt.Sprintf("option %s requires %s", optByID[r.OptionID].Code, optByID[r.RelatedOptionID].Code))
			}
		case "excludes":
			if selected[r.OptionID] && selected[r.RelatedOptionID] {
				res.Errors = append(res.Errors, fmt.Sprintf("option %s excludes %s", optByID[r.OptionID].Code, optByID[r.RelatedOptionID].Code))
			}
		}
	}

	// Price = base + Σ option deltas (deterministic order for stable output).
	ids := make([]int64, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := []string{nonEmpty(def.BasePrice)}
	for _, id := range ids {
		o := optByID[id]
		parts = append(parts, nonEmpty(o.PriceDelta))
		res.Selected = append(res.Selected, SelectedOption{
			OptionID: o.ID, GroupCode: groupByID[o.GroupID].Code, OptionCode: o.Code,
			Name: o.Name, PriceDelta: nonEmpty(o.PriceDelta),
		})
	}

	res.Valid = len(res.Errors) == 0
	if res.Valid {
		total, err := money.Sum(parts...)
		if err != nil {
			res.Valid = false
			res.Errors = append(res.Errors, "price computation failed: "+err.Error())
		} else {
			res.UnitPrice = total
		}
	}
	return res
}

func nonEmpty(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
