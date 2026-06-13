// Package exports is the data-export center: it streams full-record exports of
// the core business entities (orders, order line items, customers, invoices) as
// CSV or XLSX for the customer's own finance/BI/budgeting systems. Distinct from
// the report builder (which aggregates) — this dumps raw rows, joined to
// human-readable names, org-scoped and capped.
//
// A manifest endpoint advertises the datasets the caller may export; each
// download is gated by that entity's own view permission (least privilege), not
// a single blanket one.
package exports

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/export"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// maxExportRows caps a single synchronous export. Beyond this, callers should
// filter (or use the report builder's scheduled exports). Generous enough for
// industrial catalogs while keeping one request bounded.
const maxExportRows = 100000

type dataset struct {
	key, label, description, permission string
	fetch                               func(r *http.Request, q *gen.Queries, org int64, lim int32) (export.Table, error)
}

// order is the stable manifest ordering.
var order = []string{"orders", "order-items", "customers", "invoices"}

type Handler struct {
	pool     *pgxpool.Pool
	q        *gen.Queries
	datasets map[string]dataset
}

func New(pool *pgxpool.Pool) *Handler {
	h := &Handler{pool: pool, q: gen.New(pool)}
	h.datasets = map[string]dataset{
		"orders": {
			key: "orders", label: "Orders", description: "Order headers with totals and customer.",
			permission: "order.view", fetch: fetchOrders,
		},
		"order-items": {
			key: "order-items", label: "Order line items", description: "Every order line — SKU, quantity, price — for spend analysis.",
			permission: "order.view", fetch: fetchOrderItems,
		},
		"customers": {
			key: "customers", label: "Customers", description: "Accounts with terms, credit limit and group.",
			permission: "customer.view", fetch: fetchCustomers,
		},
		"invoices": {
			key: "invoices", label: "Invoices", description: "Invoices with status, totals and due dates.",
			permission: "invoice.view", fetch: fetchInvoices,
		},
	}
	return h
}

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))
		// Seeing the export center needs the reporting/data permission; each
		// download is additionally gated by the dataset's own entity permission.
		ar.With(mw.RequirePermission("report.view")).Get("/admin/exports", h.manifest)
		ar.With(mw.RequirePermission("report.view")).Get("/admin/exports/{dataset}", h.download)
	})
}

func (h *Handler) manifest(w http.ResponseWriter, r *http.Request) {
	claims, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	items := make([]map[string]any, 0, len(order))
	for _, key := range order {
		ds := h.datasets[key]
		if !can(claims.Permissions, ds.permission) {
			continue // hide datasets the caller can't access
		}
		items = append(items, map[string]any{
			"key": ds.key, "label": ds.label, "description": ds.description,
			"formats": []string{"csv", "xlsx"},
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"datasets": items})
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	claims, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	ds, found := h.datasets[chi.URLParam(r, "dataset")]
	if !found {
		response.Fail(w, http.StatusNotFound, "not_found", "unknown dataset")
		return
	}
	// Least privilege: the dataset's own entity permission, not a blanket one.
	if !can(claims.Permissions, ds.permission) {
		response.Fail(w, http.StatusForbidden, "forbidden", "missing "+ds.permission)
		return
	}

	lim := int32(maxExportRows)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && int32(n) < lim {
			lim = int32(n)
		}
	}
	table, err := ds.fetch(r, h.q, claims.OrgID, lim)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not build export")
		return
	}

	format := r.URL.Query().Get("format")
	stamp := time.Now().UTC().Format("20060102")
	if format == "xlsx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+ds.key+"-"+stamp+".xlsx\"")
		if err := export.WriteXLSX(w, table, ds.label); err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not write xlsx")
		}
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+ds.key+"-"+stamp+".csv\"")
	if err := export.WriteCSV(w, table); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not write csv")
	}
}

// can reports whether the permission set includes perm (empty perm = allowed).
func can(perms []string, perm string) bool {
	if perm == "" {
		return true
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// ---- dataset fetchers (gen rows → export.Table) ---------------------------

func fetchOrders(r *http.Request, q *gen.Queries, org int64, lim int32) (export.Table, error) {
	rows, err := q.ExportOrders(r.Context(), gen.ExportOrdersParams{OrganizationID: org, Limit: lim})
	if err != nil {
		return export.Table{}, err
	}
	t := export.Table{Columns: []export.Column{
		{Name: "order_id"}, {Name: "customer"}, {Name: "status"}, {Name: "currency"}, {Name: "po_number"},
		{Name: "subtotal", Numeric: true}, {Name: "tax_total", Numeric: true},
		{Name: "shipping_total", Numeric: true}, {Name: "grand_total", Numeric: true}, {Name: "created_at"},
	}}
	for _, o := range rows {
		t.Rows = append(t.Rows, []string{
			o.PublicID.String(), o.Customer, o.Status, o.Currency, deref(o.PoNumber),
			o.Subtotal, o.TaxTotal, o.ShippingTotal, o.GrandTotal, o.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return t, nil
}

func fetchOrderItems(r *http.Request, q *gen.Queries, org int64, lim int32) (export.Table, error) {
	rows, err := q.ExportOrderItems(r.Context(), gen.ExportOrderItemsParams{OrganizationID: org, Limit: lim})
	if err != nil {
		return export.Table{}, err
	}
	t := export.Table{Columns: []export.Column{
		{Name: "order_id"}, {Name: "customer"}, {Name: "sku"}, {Name: "product"},
		{Name: "quantity", Numeric: true}, {Name: "unit"}, {Name: "unit_price", Numeric: true},
		{Name: "tax_amount", Numeric: true}, {Name: "row_total", Numeric: true},
		{Name: "order_status"}, {Name: "order_date"},
	}}
	for _, it := range rows {
		t.Rows = append(t.Rows, []string{
			it.OrderPublicID.String(), it.Customer, it.Sku, it.Name,
			it.Quantity, it.Unit, it.UnitPrice, it.TaxAmount, it.RowTotal,
			it.OrderStatus, it.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return t, nil
}

func fetchCustomers(r *http.Request, q *gen.Queries, org int64, lim int32) (export.Table, error) {
	rows, err := q.ExportCustomers(r.Context(), gen.ExportCustomersParams{OrganizationID: org, Limit: lim})
	if err != nil {
		return export.Table{}, err
	}
	t := export.Table{Columns: []export.Column{
		{Name: "customer_id"}, {Name: "name"}, {Name: "tax_id"},
		{Name: "payment_terms_days", Numeric: true}, {Name: "credit_limit", Numeric: true},
		{Name: "group"}, {Name: "active"}, {Name: "created_at"},
	}}
	for _, c := range rows {
		t.Rows = append(t.Rows, []string{
			c.PublicID.String(), c.Name, deref(c.TaxID),
			strconv.Itoa(int(c.PaymentTermsDays)), c.CreditLimit,
			c.CustomerGroup, strconv.FormatBool(c.IsActive), c.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return t, nil
}

func fetchInvoices(r *http.Request, q *gen.Queries, org int64, lim int32) (export.Table, error) {
	rows, err := q.ExportInvoices(r.Context(), gen.ExportInvoicesParams{OrganizationID: org, Limit: lim})
	if err != nil {
		return export.Table{}, err
	}
	t := export.Table{Columns: []export.Column{
		{Name: "invoice_id"}, {Name: "customer"}, {Name: "status"}, {Name: "currency"},
		{Name: "subtotal", Numeric: true}, {Name: "tax_total", Numeric: true}, {Name: "grand_total", Numeric: true},
		{Name: "issued_at"}, {Name: "due_at"}, {Name: "created_at"},
	}}
	for _, inv := range rows {
		t.Rows = append(t.Rows, []string{
			inv.PublicID.String(), inv.Customer, inv.Status, inv.Currency,
			inv.Subtotal, inv.TaxTotal, inv.GrandTotal,
			tsz(inv.IssuedAt), tsz(inv.DueAt), inv.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return t, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func tsz(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}
