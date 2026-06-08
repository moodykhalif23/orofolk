package reporting_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/report"
	"b2bcommerce/internal/store/gen"
)

// seedTwoOrders creates two non-cancelled orders (100 + 50) sharing one product,
// so the aggregate report sums to 150 across 2 rows.
func seedTwoOrders(t *testing.T, pool *pgxpool.Pool, org int64) {
	t.Helper()
	q := gen.New(pool)
	ctx := context.Background()
	cust, err := q.CreateCustomer(ctx, gen.CreateCustomerParams{OrganizationID: org, Name: "Acme RB", CreditLimit: "0"})
	if err != nil {
		t.Fatalf("customer: %v", err)
	}
	p, err := q.CreateProduct(ctx, gen.CreateProductParams{OrganizationID: org, Sku: "RB1", Type: "simple", Name: "RB", Slug: "rb1", Status: "active", Attributes: []byte("{}"), Unit: "each"})
	if err != nil {
		t.Fatalf("product: %v", err)
	}
	for _, amt := range []string{"100.0000", "50.0000"} {
		o, err := q.CreateOrder(ctx, gen.CreateOrderParams{
			OrganizationID: org, WebsiteID: 1, CustomerID: cust.ID, Currency: "USD",
			BillingAddress: []byte("{}"), ShippingAddress: []byte("{}"),
			Subtotal: amt, TaxTotal: "0", ShippingTotal: "0", GrandTotal: amt,
		})
		if err != nil {
			t.Fatalf("order: %v", err)
		}
		if _, err := q.AddOrderItem(ctx, gen.AddOrderItemParams{
			OrderID: o.ID, ProductID: p.ID, Sku: p.Sku, Name: p.Name,
			Quantity: "1", Unit: "each", UnitPrice: amt, TaxAmount: "0", RowTotal: amt,
		}); err != nil {
			t.Fatalf("order item: %v", err)
		}
	}
}

func manageToken(t *testing.T, issuer *auth.Issuer) string {
	t.Helper()
	tok, _ := issuer.Issue("1", 1, "admin", []string{"report.view", "report.manage"})
	return tok
}

// fakeMailer records report-ready notifications for the schedule test.
type fakeMailer struct{ sent []string }

func (m *fakeMailer) EnqueueEmail(_ context.Context, to, _ string, _ map[string]any) error {
	m.sent = append(m.sent, to)
	return nil
}

func TestReportBuilderFlow(t *testing.T) {
	h, issuer, pool := newServer(t)
	tok := manageToken(t, issuer)
	seedTwoOrders(t, pool, 1)

	// Entity metadata is discoverable.
	var ents struct {
		Entities map[string]report.EntitySchema `json:"entities"`
	}
	decode(t, do(t, h, http.MethodGet, "/admin/reports/entities", tok, nil), &ents)
	if _, ok := ents.Entities["orders"]; !ok {
		t.Fatal("entities should include orders")
	}

	// Unknown measure is rejected at save (not at run).
	bad := map[string]any{"name": "bad", "entity": "orders", "measures": []map[string]any{{"field": "password", "agg": "sum"}}}
	if rr := do(t, h, http.MethodPost, "/admin/reports", tok, bad); rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown measure: want 400, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Create a valid definition: total revenue + order count.
	body := map[string]any{
		"name":   "Revenue",
		"entity": "orders",
		"measures": []map[string]any{
			{"field": "grand_total", "agg": "sum"},
			{"agg": "count"},
		},
	}
	var def struct {
		ID int64 `json:"id"`
	}
	rr := do(t, h, http.MethodPost, "/admin/reports", tok, body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create def: want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	decode(t, rr, &def)

	// Run it: one aggregate row, revenue summed across the two orders.
	var run struct {
		Columns  []string    `json:"columns"`
		Rows     [][]*string `json:"rows"`
		RowCount int         `json:"row_count"`
		RunID    int64       `json:"run_id"`
	}
	rr = do(t, h, http.MethodPost, "/admin/reports/"+itoa(def.ID)+"/run", tok, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("run: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	decode(t, rr, &run)
	if len(run.Columns) != 2 || run.Columns[0] != "grand_total_sum" || run.Columns[1] != "count" {
		t.Fatalf("columns = %v", run.Columns)
	}
	if len(run.Rows) != 1 || run.Rows[0][0] == nil || *run.Rows[0][0] != "150.0000" || *run.Rows[0][1] != "2" {
		t.Fatalf("rows = %v", run.Rows)
	}

	// A run was recorded.
	var runs struct {
		Items []struct {
			Status  string `json:"status"`
			Trigger string `json:"trigger"`
		} `json:"items"`
	}
	decode(t, do(t, h, http.MethodGet, "/admin/reports/"+itoa(def.ID)+"/runs", tok, nil), &runs)
	if len(runs.Items) == 0 || runs.Items[0].Status != "ok" {
		t.Fatalf("runs = %+v", runs.Items)
	}

	// CSV run streams a file.
	csv := do(t, h, http.MethodPost, "/admin/reports/"+itoa(def.ID)+"/run?format=csv", tok, nil)
	if csv.Code != http.StatusOK || csv.Header().Get("Content-Type") != "text/csv" {
		t.Fatalf("csv run: %d type=%s", csv.Code, csv.Header().Get("Content-Type"))
	}
	if !contains(csv.Body.String(), "grand_total_sum") {
		t.Errorf("csv missing header: %s", csv.Body.String())
	}

	// Schedule it daily, then run the due-schedule sweep directly.
	sched := map[string]any{"cadence": "daily", "recipients": []string{"ops@acme.test"}}
	if rr := do(t, h, http.MethodPost, "/admin/reports/"+itoa(def.ID)+"/schedules", tok, sched); rr.Code != http.StatusCreated {
		t.Fatalf("create schedule: want 201, got %d (%s)", rr.Code, rr.Body.String())
	}

	mailer := &fakeMailer{}
	n, err := report.RunDueSchedules(context.Background(), pool, mailer, time.Now())
	if err != nil {
		t.Fatalf("RunDueSchedules: %v", err)
	}
	if n != 1 {
		t.Fatalf("due schedules processed = %d, want 1", n)
	}
	if len(mailer.sent) != 1 || mailer.sent[0] != "ops@acme.test" {
		t.Errorf("recipients notified = %v", mailer.sent)
	}

	// The schedule produced a downloadable artifact.
	decode(t, do(t, h, http.MethodGet, "/admin/reports/"+itoa(def.ID)+"/runs", tok, nil), &runs)
	var fileURL string
	{
		var detail struct {
			Items []struct {
				Trigger string  `json:"trigger"`
				FileURL *string `json:"file_url"`
			} `json:"items"`
		}
		decode(t, do(t, h, http.MethodGet, "/admin/reports/"+itoa(def.ID)+"/runs", tok, nil), &detail)
		for _, it := range detail.Items {
			if it.Trigger == "schedule" && it.FileURL != nil {
				fileURL = *it.FileURL
			}
		}
	}
	if fileURL == "" {
		t.Fatal("scheduled run produced no file_url")
	}
	dl := do(t, h, http.MethodGet, fileURL, tok, nil)
	if dl.Code != http.StatusOK || !contains(dl.Body.String(), "150.0000") {
		t.Fatalf("download: %d body=%s", dl.Code, dl.Body.String())
	}

	// A second sweep is a no-op (cadence not yet elapsed).
	n2, _ := report.RunDueSchedules(context.Background(), pool, mailer, time.Now())
	if n2 != 0 {
		t.Errorf("second sweep processed %d, want 0 (cadence not elapsed)", n2)
	}
}

func TestReportBuilderRequiresManage(t *testing.T) {
	h, issuer, _ := newServer(t)
	viewTok := reportToken(t, issuer) // report.view only
	body := map[string]any{"name": "x", "entity": "orders", "measures": []map[string]any{{"agg": "count"}}}
	if rr := do(t, h, http.MethodPost, "/admin/reports", viewTok, body); rr.Code != http.StatusForbidden {
		t.Errorf("create without report.manage: want 403, got %d", rr.Code)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i, neg := len(b), n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
