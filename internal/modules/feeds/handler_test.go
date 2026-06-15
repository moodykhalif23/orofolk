package feeds_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/blob"
	"b2bcommerce/internal/server"
	"b2bcommerce/internal/store"
	"b2bcommerce/internal/testsupport"
)

const testSecret = "test-secret-please-change"

func newServer(t *testing.T) (http.Handler, string, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewDB(t)
	st := store.New(pool)
	issuer := auth.NewIssuer(testSecret, time.Hour)
	tok, err := issuer.Issue("1", 1, "admin", []string{"feed.view", "feed.manage"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	// A real blob store so build + signed delivery work end to end.
	bs, err := blob.NewFSStore(t.TempDir())
	if err != nil {
		t.Fatalf("blob store: %v", err)
	}
	return server.New(st, issuer, server.WithMedia(bs, nil)), tok, pool
}

// doPublic issues an unauthenticated GET (the channel polling a signed feed URL).
func doPublic(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func doJSON(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// TestFeedObjectLifecycle drives a feed over a custom object type end to end:
// create with a mapping (source field + constant), preview, generate CSV then
// JSON output, then delete.
func TestFeedObjectLifecycle(t *testing.T) {
	h, token, pool := newServer(t)
	ctx := context.Background()

	var typeID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO object_types (organization_id, code, label) VALUES (1, 'feed_supplier', 'Supplier') RETURNING id`).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_fields (object_type_id, organization_id, code, label, data_type, sort_order)
		 VALUES ($1, 1, 'name', 'Name', 'text', 0), ($1, 1, 'tier', 'Tier', 'number', 1)`, typeID); err != nil {
		t.Fatalf("seed fields: %v", err)
	}
	for _, d := range []string{`{"name":"Acme","tier":2}`, `{"name":"Globex","tier":5}`} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO object_records (object_type_id, organization_id, data) VALUES ($1, 1, $2::jsonb)`, typeID, d); err != nil {
			t.Fatalf("seed record: %v", err)
		}
	}

	// Create a feed: two mapped fields + one constant column.
	create := doJSON(t, h, http.MethodPost, "/admin/feeds", token, map[string]any{
		"name":   "Supplier export",
		"source": "object:feed_supplier",
		"format": "csv",
		"mapping": []map[string]any{
			{"out": "supplier", "src": "name"},
			{"out": "level", "src": "tier"},
			{"out": "status", "const": "active"},
		},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", create.Code, create.Body.String())
	}
	var feed struct {
		ID     int64  `json:"id"`
		Format string `json:"format"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &feed)
	id := strconv.FormatInt(feed.ID, 10)

	// Preview renders the projection (CSV header + both rows + the constant).
	prev := doJSON(t, h, http.MethodGet, "/admin/feeds/"+id+"/preview", token, nil)
	if prev.Code != http.StatusOK {
		t.Fatalf("preview: status %d, body %s", prev.Code, prev.Body.String())
	}
	var pv struct {
		Rows    int    `json:"rows"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(prev.Body.Bytes(), &pv)
	if pv.Rows != 2 {
		t.Errorf("preview rows=%d, want 2", pv.Rows)
	}
	if !strings.Contains(pv.Content, "supplier,level,status") {
		t.Errorf("preview missing header:\n%s", pv.Content)
	}
	if !strings.Contains(pv.Content, "Acme,2,active") || !strings.Contains(pv.Content, "Globex,5,active") {
		t.Errorf("preview missing projected rows:\n%s", pv.Content)
	}

	// CSV output is a real download.
	out := doJSON(t, h, http.MethodGet, "/admin/feeds/"+id+"/output", token, nil)
	if out.Code != http.StatusOK {
		t.Fatalf("output: status %d, body %s", out.Code, out.Body.String())
	}
	if ct := out.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("output content-type=%q, want text/csv", ct)
	}
	if !strings.Contains(out.Body.String(), "Acme,2,active") {
		t.Errorf("csv output missing row:\n%s", out.Body.String())
	}

	// Switch the feed to JSON and regenerate — the same projection, structured.
	upd := doJSON(t, h, http.MethodPut, "/admin/feeds/"+id, token, map[string]any{
		"name":   "Supplier export",
		"source": "object:feed_supplier",
		"format": "json",
		"mapping": []map[string]any{
			{"out": "supplier", "src": "name"},
			{"out": "level", "src": "tier"},
			{"out": "status", "const": "active"},
		},
	})
	if upd.Code != http.StatusOK {
		t.Fatalf("update: status %d, body %s", upd.Code, upd.Body.String())
	}
	jout := doJSON(t, h, http.MethodGet, "/admin/feeds/"+id+"/output", token, nil)
	if ct := jout.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("json output content-type=%q, want application/json", ct)
	}
	var arr []map[string]any
	if err := json.Unmarshal(jout.Body.Bytes(), &arr); err != nil {
		t.Fatalf("json output not valid: %v\n%s", err, jout.Body.String())
	}
	if len(arr) != 2 {
		t.Fatalf("json output len=%d, want 2", len(arr))
	}
	if arr[0]["status"] != "active" || (arr[0]["supplier"] != "Acme" && arr[0]["supplier"] != "Globex") {
		t.Errorf("json row 0 = %v", arr[0])
	}

	// Delete, then it's gone.
	if del := doJSON(t, h, http.MethodDelete, "/admin/feeds/"+id, token, nil); del.Code != http.StatusOK {
		t.Errorf("delete: status %d", del.Code)
	}
	if g := doJSON(t, h, http.MethodGet, "/admin/feeds/"+id, token, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete: status %d, want 404", g.Code)
	}
}

// TestFeedProductSourceFlattensAttributes proves the products source exposes
// attr.<code> fields, so a feed can project a product attribute into a channel
// column. Also checks source discovery and unknown-source rejection.
func TestFeedProductSourceFlattensAttributes(t *testing.T) {
	h, token, pool := newServer(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO products (organization_id, sku, type, name, slug, status, attributes)
		 VALUES (1, 'FEED-RED', 'simple', 'Red Widget', 'feed-red', 'active', '{"color":"red"}'::jsonb)`); err != nil {
		t.Fatalf("seed product: %v", err)
	}

	// Discovery lists products as a source.
	src := doJSON(t, h, http.MethodGet, "/admin/feeds/sources", token, nil)
	if src.Code != http.StatusOK || !strings.Contains(src.Body.String(), `"products"`) {
		t.Fatalf("sources: status %d, body %s", src.Code, src.Body.String())
	}

	// Unknown source is rejected at create.
	bad := doJSON(t, h, http.MethodPost, "/admin/feeds", token, map[string]any{"name": "x", "source": "object:nope"})
	if bad.Code != http.StatusBadRequest {
		t.Errorf("create with unknown source = %d, want 400", bad.Code)
	}

	create := doJSON(t, h, http.MethodPost, "/admin/feeds", token, map[string]any{
		"name":   "Shopping feed",
		"source": "products",
		"format": "json",
		"mapping": []map[string]any{
			{"out": "id", "src": "sku"},
			{"out": "title", "src": "name"},
			{"out": "color", "src": "attr.color"},
		},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", create.Code, create.Body.String())
	}
	var feed struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &feed)

	out := doJSON(t, h, http.MethodGet, "/admin/feeds/"+strconv.FormatInt(feed.ID, 10)+"/output", token, nil)
	if out.Code != http.StatusOK {
		t.Fatalf("output: status %d, body %s", out.Code, out.Body.String())
	}
	var arr []map[string]any
	if err := json.Unmarshal(out.Body.Bytes(), &arr); err != nil {
		t.Fatalf("json output invalid: %v", err)
	}
	// org 1 is the demo org, so the catalog holds other products too; find ours.
	var found map[string]any
	for _, row := range arr {
		if row["id"] == "FEED-RED" {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatalf("FEED-RED not in product feed (%d rows)", len(arr))
	}
	if found["title"] != "Red Widget" || found["color"] != "red" {
		t.Errorf("FEED-RED row = %v, want title 'Red Widget' + color 'red' (per-row attribute flatten)", found)
	}
}

// TestFeedGoogleShoppingChannel proves the channel adapter (slice 2): the
// google_shopping channel pins XML, renders the RSS 2.0 / g-namespace envelope,
// and the gap check flags g:price (required by the channel, absent from the
// product source) so the author knows to map it.
func TestFeedGoogleShoppingChannel(t *testing.T) {
	h, token, pool := newServer(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO products (organization_id, sku, type, name, slug, status, attributes)
		 VALUES (1, 'GFEED-1', 'simple', 'Gadget', 'gfeed-1', 'active', '{}'::jsonb)`); err != nil {
		t.Fatalf("seed product: %v", err)
	}

	// The registry advertises google_shopping with g:price required.
	chs := doJSON(t, h, http.MethodGet, "/admin/feeds/channels", token, nil)
	if chs.Code != http.StatusOK {
		t.Fatalf("channels: status %d, body %s", chs.Code, chs.Body.String())
	}
	if !strings.Contains(chs.Body.String(), `"google_shopping"`) || !strings.Contains(chs.Body.String(), `"g:price"`) {
		t.Errorf("channels list missing google_shopping/g:price:\n%s", chs.Body.String())
	}

	// Unknown channel is rejected.
	if bad := doJSON(t, h, http.MethodPost, "/admin/feeds", token, map[string]any{
		"name": "x", "source": "products", "channel": "nope",
	}); bad.Code != http.StatusBadRequest {
		t.Errorf("unknown channel create = %d, want 400", bad.Code)
	}

	// Create a Google Shopping feed WITHOUT g:price; format=csv must be overridden
	// to the channel's pinned xml.
	create := doJSON(t, h, http.MethodPost, "/admin/feeds", token, map[string]any{
		"name":    "Google feed",
		"source":  "products",
		"channel": "google_shopping",
		"format":  "csv",
		"mapping": []map[string]any{
			{"out": "g:id", "src": "sku"},
			{"out": "title", "src": "name"},
			{"out": "description", "src": "description"},
			{"out": "link", "src": "slug"},
			{"out": "g:image_link", "src": "image_url"},
			{"out": "g:availability", "const": "in_stock"},
			{"out": "g:condition", "const": "new"},
		},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", create.Code, create.Body.String())
	}
	var feed struct {
		ID     int64  `json:"id"`
		Format string `json:"format"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &feed)
	if feed.Format != "xml" {
		t.Errorf("stored format=%q, want xml (channel pins it)", feed.Format)
	}
	id := strconv.FormatInt(feed.ID, 10)

	// Preview flags the missing required field.
	prev := doJSON(t, h, http.MethodGet, "/admin/feeds/"+id+"/preview", token, nil)
	var pv struct {
		Format          string   `json:"format"`
		Channel         string   `json:"channel"`
		MissingRequired []string `json:"missing_required"`
	}
	_ = json.Unmarshal(prev.Body.Bytes(), &pv)
	if pv.Format != "xml" || pv.Channel != "google_shopping" {
		t.Errorf("preview format=%q channel=%q, want xml/google_shopping", pv.Format, pv.Channel)
	}
	if !contains(pv.MissingRequired, "g:price") {
		t.Errorf("missing_required=%v, want it to flag g:price", pv.MissingRequired)
	}

	// Output is real RSS 2.0 with the g: namespace and our product.
	out := doJSON(t, h, http.MethodGet, "/admin/feeds/"+id+"/output", token, nil)
	if out.Code != http.StatusOK {
		t.Fatalf("output: status %d, body %s", out.Code, out.Body.String())
	}
	if ct := out.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("output content-type=%q, want application/xml", ct)
	}
	body := out.Body.String()
	for _, want := range []string{
		`<rss version="2.0" xmlns:g="http://base.google.com/ns/1.0">`,
		"<channel>",
		"<g:id>GFEED-1</g:id>",
		"<g:availability>in_stock</g:availability>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("RSS output missing %q:\n%s", want, body[:min(len(body), 600)])
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestFeedScheduledBuildAndSignedDelivery proves slice 3: a scheduled feed is
// armed with a next run, a build stores an artifact, and the signed delivery URL
// serves it to an unauthenticated caller — while a tampered signature and an
// unbuilt feed are refused.
func TestFeedScheduledBuildAndSignedDelivery(t *testing.T) {
	h, token, pool := newServer(t)
	ctx := context.Background()

	var typeID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO object_types (organization_id, code, label) VALUES (1, 'feed_sched', 'Sched') RETURNING id`).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_fields (object_type_id, organization_id, code, label, data_type, sort_order)
		 VALUES ($1, 1, 'name', 'Name', 'text', 0)`, typeID); err != nil {
		t.Fatalf("seed field: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_records (object_type_id, organization_id, data) VALUES ($1, 1, '{"name":"Acme"}'::jsonb)`, typeID); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	// A daily-scheduled feed is armed with a next run.
	create := doJSON(t, h, http.MethodPost, "/admin/feeds", token, map[string]any{
		"name": "Sched feed", "source": "object:feed_sched", "format": "csv", "schedule": "daily",
		"mapping": []map[string]any{{"out": "supplier", "src": "name"}},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", create.Code, create.Body.String())
	}
	var fv struct {
		ID        int64  `json:"id"`
		URL       string `json:"url"`
		Schedule  string `json:"schedule"`
		NextRunAt string `json:"next_run_at"`
	}
	_ = json.Unmarshal(create.Body.Bytes(), &fv)
	if fv.Schedule != "daily" || fv.NextRunAt == "" {
		t.Errorf("scheduled feed not armed: schedule=%q next_run_at=%q", fv.Schedule, fv.NextRunAt)
	}
	if fv.URL == "" {
		t.Fatal("feed view returned no signed url")
	}
	id := strconv.FormatInt(fv.ID, 10)

	// Before any build, the signed URL has nothing to serve.
	if pre := doPublic(t, h, fv.URL); pre.Code != http.StatusNotFound {
		t.Errorf("delivery before build = %d, want 404", pre.Code)
	}

	// Build now (the same path the scheduler runs).
	build := doJSON(t, h, http.MethodPost, "/admin/feeds/"+id+"/build", token, nil)
	if build.Code != http.StatusOK {
		t.Fatalf("build: status %d, body %s", build.Code, build.Body.String())
	}
	var bv struct {
		URL         string `json:"url"`
		LastBuiltAt string `json:"last_built_at"`
		LastBytes   int64  `json:"last_bytes"`
	}
	_ = json.Unmarshal(build.Body.Bytes(), &bv)
	if bv.LastBuiltAt == "" || bv.LastBytes == 0 {
		t.Errorf("build didn't stamp metadata: last_built_at=%q last_bytes=%d", bv.LastBuiltAt, bv.LastBytes)
	}

	// The signed URL now serves the stored artifact to an anonymous caller.
	got := doPublic(t, h, bv.URL)
	if got.Code != http.StatusOK {
		t.Fatalf("signed delivery: status %d, body %s", got.Code, got.Body.String())
	}
	if ct := got.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("delivery content-type=%q, want text/csv", ct)
	}
	if body := got.Body.String(); !strings.Contains(body, "supplier") || !strings.Contains(body, "Acme") {
		t.Errorf("delivered artifact missing data:\n%s", body)
	}

	// A tampered signature is refused.
	if tam := doPublic(t, h, strings.Replace(bv.URL, "sig=", "sig=00", 1)); tam.Code != http.StatusForbidden {
		t.Errorf("tampered signature = %d, want 403", tam.Code)
	}
}
