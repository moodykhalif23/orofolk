package demo

import (
	"context"
	"math/big"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/money"
	"b2bcommerce/internal/store/gen"
)

// demoProduct is one catalog line to seed (sale price + cost so margin works).
type demoProduct struct {
	sku, name, slug, price, cost string
}

var demoProducts = []demoProduct{
	{"CP-50", "Centrifugal Pump CP-50", "centrifugal-pump-cp-50", "940.0000", "610.0000"},
	{"MTR-5HP", "3-Phase Motor 5HP", "three-phase-motor-5hp", "510.0000", "350.0000"},
	{"PS-200", "Pressure Sensor PS-200", "pressure-sensor-ps-200", "145.0000", "88.0000"},
	{"BV-2", `Industrial Ball Valve 2"`, "industrial-ball-valve-2in", "120.0000", "78.0000"},
	{"RB-22", "Roller Bearing RB-22", "roller-bearing-rb-22", "36.0000", "19.5000"},
	{"GS-10", "Gasket Set GS-10 (pack)", "gasket-set-gs-10", "22.0000", "9.0000"},
}

var demoCustomers = []string{
	"Northwind Manufacturing", "Apex Industrial Supply", "Meridian Fabrication", "Cascade Process Co",
}

// demoOrder references products/customers by index; ageDays backdates the order
// so the insights engine has both a current and a prior trading week.
type demoOrder struct {
	customer int
	lines    []demoLine
	ageDays  int
}
type demoLine struct {
	product, qty int
}

var demoOrders = []demoOrder{
	{0, []demoLine{{0, 1}, {4, 4}}, 1},
	{1, []demoLine{{3, 6}, {5, 10}}, 2},
	{2, []demoLine{{1, 2}}, 3},
	{0, []demoLine{{2, 5}, {4, 8}}, 5},
	{3, []demoLine{{0, 1}}, 6},
	{1, []demoLine{{3, 3}, {1, 1}}, 9},   // prior week
	{2, []demoLine{{4, 20}}, 11},         // prior week
	{3, []demoLine{{5, 15}, {2, 4}}, 12}, // prior week
}

// seed populates a fresh demo org with representative catalog, customers and
// orders so the dashboard, insights and margin views are alive on first login.
// Best-effort: any single failure is skipped rather than failing the demo.
func seed(ctx context.Context, pool *pgxpool.Pool, orgID, websiteID int64) {
	q := gen.New(pool)

	prodIDs := make([]int64, len(demoProducts))
	for i, p := range demoProducts {
		created, err := q.CreateProduct(ctx, gen.CreateProductParams{
			OrganizationID: orgID, Sku: p.sku, Type: "simple", Name: p.name, Slug: p.slug,
			Status: "active", Attributes: []byte("{}"), Unit: "each",
		})
		if err != nil {
			continue
		}
		prodIDs[i] = created.ID
		_, _ = q.SetProductCost(ctx, gen.SetProductCostParams{OrganizationID: orgID, ID: created.ID, CostPrice: p.cost})
	}

	custIDs := make([]int64, len(demoCustomers))
	for i, name := range demoCustomers {
		c, err := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: orgID, Name: name, CreditLimit: "250000"})
		if err == nil {
			custIDs[i] = c.ID
		}
	}

	for _, o := range demoOrders {
		if o.customer >= len(custIDs) || custIDs[o.customer] == 0 {
			continue
		}
		// Compute the order total from its lines.
		total := "0"
		type line struct {
			prodIdx int
			qty     int
			rowTot  string
		}
		var lines []line
		for _, l := range o.lines {
			if l.product >= len(prodIDs) || prodIDs[l.product] == 0 {
				continue
			}
			rt := mul(demoProducts[l.product].price, l.qty)
			total, _ = money.Sum(total, rt)
			lines = append(lines, line{l.product, l.qty, rt})
		}
		if len(lines) == 0 {
			continue
		}
		ord, err := q.CreateOrder(ctx, gen.CreateOrderParams{
			OrganizationID: orgID, WebsiteID: websiteID, CustomerID: custIDs[o.customer], Currency: "USD",
			BillingAddress: []byte("{}"), ShippingAddress: []byte("{}"),
			Subtotal: total, TaxTotal: "0", ShippingTotal: "0", GrandTotal: total,
		})
		if err != nil {
			continue
		}
		for _, l := range lines {
			p := demoProducts[l.prodIdx]
			_, _ = q.AddOrderItem(ctx, gen.AddOrderItemParams{
				OrderID: ord.ID, ProductID: prodIDs[l.prodIdx], Sku: p.sku, Name: p.name,
				Quantity: strconv.Itoa(l.qty), Unit: "each", UnitPrice: p.price, TaxAmount: "0", RowTotal: l.rowTot,
			})
		}
		// Backdate so prior-period comparisons have data.
		if o.ageDays > 0 {
			_, _ = pool.Exec(ctx, `UPDATE orders SET created_at = now() - make_interval(days => $2) WHERE id = $1`, ord.ID, o.ageDays)
		}
	}
}

// mul returns price × qty as a 4dp money string.
func mul(price string, qty int) string {
	p, err := money.Parse(price)
	if err != nil {
		return "0"
	}
	return money.Format(new(big.Rat).Mul(p, new(big.Rat).SetInt64(int64(qty))))
}
