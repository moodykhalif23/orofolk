package imports_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/apikey"
	"b2bcommerce/internal/auth"
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
	tok, err := issuer.Issue("1", 1, "admin", []string{"import.view", "import.manage"})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return server.New(st, issuer), tok, pool
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

func doUploadCSV(t *testing.T, h http.Handler, path, token, csvBody string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "data.csv")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	_, _ = fw.Write([]byte(csvBody))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

type runResp struct {
	Run struct {
		ID         int64  `json:"id"`
		Status     string `json:"status"`
		TotalRows  int    `json:"total_rows"`
		CreateRows int    `json:"create_rows"`
		UpdateRows int    `json:"update_rows"`
		ErrorRows  int    `json:"error_rows"`
	} `json:"run"`
	Preview []map[string]any `json:"preview"`
}

// TestImportProductsDryRunThenCommit drives the engine end-to-end on the product
// target: a CSV with an update, a create and a bad row is staged (dry run), then
// committed — and only the two valid rows land.
func TestImportProductsDryRunThenCommit(t *testing.T) {
	h, token, pool := newServer(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO products (organization_id, sku, name, slug, status) VALUES (1, 'IMP-EXIST', 'Old Name', 'imp-exist', 'active')`); err != nil {
		t.Fatalf("seed product: %v", err)
	}

	csv := "sku,name,slug,cost_price\n" +
		"IMP-EXIST,New Name,imp-exist,5\n" +
		"IMP-NEW,Fresh,imp-new,3\n" +
		"IMP-BAD,,imp-bad,\n"
	rr := doUploadCSV(t, h, "/admin/imports?target=products", token, csv)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload: status %d, body %s", rr.Code, rr.Body.String())
	}
	var res runResp
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.Run.TotalRows != 3 || res.Run.CreateRows != 1 || res.Run.UpdateRows != 1 || res.Run.ErrorRows != 1 {
		t.Fatalf("dry-run counts total=%d create=%d update=%d error=%d, want 3/1/1/1",
			res.Run.TotalRows, res.Run.CreateRows, res.Run.UpdateRows, res.Run.ErrorRows)
	}
	if len(res.Preview) != 3 {
		t.Errorf("preview len=%d, want 3", len(res.Preview))
	}

	// Nothing applied yet: the new product must not exist before commit.
	var pre int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE organization_id=1 AND sku='IMP-NEW'`).Scan(&pre)
	if pre != 0 {
		t.Fatalf("IMP-NEW exists before commit (%d) — dry run wrote to the target", pre)
	}

	commit := doJSON(t, h, http.MethodPost, "/admin/imports/runs/"+strconv.FormatInt(res.Run.ID, 10)+"/commit", token, nil)
	if commit.Code != http.StatusOK {
		t.Fatalf("commit: status %d, body %s", commit.Code, commit.Body.String())
	}
	var cres struct {
		Committed int `json:"committed"`
	}
	_ = json.Unmarshal(commit.Body.Bytes(), &cres)
	if cres.Committed != 2 {
		t.Errorf("committed=%d, want 2", cres.Committed)
	}

	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM products WHERE organization_id=1 AND sku='IMP-EXIST'`).Scan(&name); err != nil {
		t.Fatalf("read updated: %v", err)
	}
	if name != "New Name" {
		t.Errorf("IMP-EXIST name=%q, want updated to %q", name, "New Name")
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE organization_id=1 AND sku='IMP-NEW'`).Scan(&n)
	if n != 1 {
		t.Errorf("IMP-NEW count=%d after commit, want 1", n)
	}

	// A second commit is rejected (already committed).
	if again := doJSON(t, h, http.MethodPost, "/admin/imports/runs/"+strconv.FormatInt(res.Run.ID, 10)+"/commit", token, nil); again.Code != http.StatusConflict {
		t.Errorf("re-commit: status %d, want 409", again.Code)
	}
}

// TestImportObjectRecordsValidates proves the generic target + validation reuse:
// importing custom object records rejects a value that breaks the field's rule.
func TestImportObjectRecordsValidates(t *testing.T) {
	h, token, pool := newServer(t)
	ctx := context.Background()

	var typeID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO object_types (organization_id, code, label) VALUES (1, 'imp_supplier', 'Supplier') RETURNING id`).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_fields (object_type_id, organization_id, code, label, data_type, validation)
		 VALUES ($1, 1, 'rating', 'Rating', 'number', '{"max":5}'::jsonb)`, typeID); err != nil {
		t.Fatalf("seed field: %v", err)
	}

	body := []map[string]any{{"rating": 3}, {"rating": 9}}
	rr := doJSON(t, h, http.MethodPost, "/admin/imports?target=object:imp_supplier&format=json", token, body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload: status %d, body %s", rr.Code, rr.Body.String())
	}
	var res runResp
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.Run.CreateRows != 1 || res.Run.ErrorRows != 1 {
		t.Fatalf("object dry-run create=%d error=%d, want 1/1; body %s", res.Run.CreateRows, res.Run.ErrorRows, rr.Body.String())
	}

	commit := doJSON(t, h, http.MethodPost, "/admin/imports/runs/"+strconv.FormatInt(res.Run.ID, 10)+"/commit", token, nil)
	if commit.Code != http.StatusOK {
		t.Fatalf("commit: status %d, body %s", commit.Code, commit.Body.String())
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM object_records WHERE organization_id=1 AND object_type_id=$1 AND deleted_at IS NULL`, typeID).Scan(&n)
	if n != 1 {
		t.Errorf("object records after commit=%d, want 1 (only the valid row)", n)
	}
}

// TestImportObjectUpsertByMatchField proves slice-3 matching: importing the same
// key twice updates the existing record instead of duplicating it.
func TestImportObjectUpsertByMatchField(t *testing.T) {
	h, token, pool := newServer(t)
	ctx := context.Background()

	var typeID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO object_types (organization_id, code, label) VALUES (1, 'imp_cust', 'Customer') RETURNING id`).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_fields (object_type_id, organization_id, code, label, data_type, sort_order)
		 VALUES ($1, 1, 'email', 'Email', 'text', 0), ($1, 1, 'name', 'Name', 'text', 1)`, typeID); err != nil {
		t.Fatalf("seed fields: %v", err)
	}

	commitRun := func(id int64) {
		c := doJSON(t, h, http.MethodPost, "/admin/imports/runs/"+strconv.FormatInt(id, 10)+"/commit", token, nil)
		if c.Code != http.StatusOK {
			t.Fatalf("commit: status %d, body %s", c.Code, c.Body.String())
		}
	}

	// First import creates the record.
	up1 := doJSON(t, h, http.MethodPost, "/admin/imports?target=object:imp_cust&format=json&match=email", token,
		[]map[string]any{{"email": "a@x.com", "name": "Acme"}})
	var r1 runResp
	_ = json.Unmarshal(up1.Body.Bytes(), &r1)
	if r1.Run.CreateRows != 1 {
		t.Fatalf("first import create=%d, want 1; body %s", r1.Run.CreateRows, up1.Body.String())
	}
	commitRun(r1.Run.ID)

	// Second import with the same email upserts (update, not a duplicate).
	up2 := doJSON(t, h, http.MethodPost, "/admin/imports?target=object:imp_cust&format=json&match=email", token,
		[]map[string]any{{"email": "a@x.com", "name": "Acme Corp"}})
	var r2 runResp
	_ = json.Unmarshal(up2.Body.Bytes(), &r2)
	if r2.Run.UpdateRows != 1 {
		t.Fatalf("second import update=%d, want 1; body %s", r2.Run.UpdateRows, up2.Body.String())
	}
	commitRun(r2.Run.ID)

	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM object_records WHERE organization_id=1 AND object_type_id=$1 AND deleted_at IS NULL`, typeID).Scan(&n)
	if n != 1 {
		t.Errorf("records=%d after upsert, want 1 (matched, not duplicated)", n)
	}
	var name string
	_ = pool.QueryRow(ctx, `SELECT data->>'name' FROM object_records WHERE organization_id=1 AND object_type_id=$1 AND deleted_at IS NULL`, typeID).Scan(&name)
	if name != "Acme Corp" {
		t.Errorf("name=%q, want updated to %q", name, "Acme Corp")
	}
}

// TestIngestWithAPIKey is the slice-4 headline: a supplier authenticates with a
// programmatic key scoped to ONLY import.ingest and feeds records in one call —
// validated, applied and recorded with no separate commit. It also proves least
// privilege (the key can't browse runs) and that discovery stays reachable.
func TestIngestWithAPIKey(t *testing.T) {
	h, _, pool := newServer(t) // the JWT is unused here — we authenticate with an API key
	ctx := context.Background()

	var typeID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO object_types (organization_id, code, label) VALUES (1, 'imp_partner', 'Partner') RETURNING id`).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_fields (object_type_id, organization_id, code, label, data_type, validation, sort_order)
		 VALUES ($1, 1, 'email', 'Email', 'text', '{}'::jsonb, 0), ($1, 1, 'tier', 'Tier', 'number', '{"max":3}'::jsonb, 1)`, typeID); err != nil {
		t.Fatalf("seed fields: %v", err)
	}

	// Mint a supplier key carrying ONLY import.ingest.
	raw, prefix, hash, err := apikey.Generate()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO api_keys (organization_id, name, prefix, key_hash, scopes) VALUES (1, 'Supplier feed', $1, $2, $3)`,
		prefix, hash, []string{"import.ingest"}); err != nil {
		t.Fatalf("seed key: %v", err)
	}

	// One valid row, one the field rule rejects — fed in a single call as the key.
	body := []map[string]any{{"email": "a@x.com", "tier": 2}, {"email": "b@x.com", "tier": 9}}
	rr := doJSON(t, h, http.MethodPost, "/admin/imports/ingest?target=object:imp_partner&format=json&match=email", raw, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("ingest: status %d, body %s", rr.Code, rr.Body.String())
	}
	var res struct {
		Applied int `json:"applied"`
		Created int `json:"created"`
		Errors  int `json:"errors"`
		Run     struct {
			Status string `json:"status"`
		} `json:"run"`
		Results []map[string]any `json:"results"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.Applied != 1 || res.Created != 1 || res.Errors != 1 {
		t.Fatalf("ingest applied=%d created=%d errors=%d, want 1/1/1; body %s", res.Applied, res.Created, res.Errors, rr.Body.String())
	}
	if res.Run.Status != "committed" {
		t.Errorf("run status=%q, want committed (applied in one call)", res.Run.Status)
	}
	if len(res.Results) != 2 {
		t.Errorf("results len=%d, want 2 (one per row)", len(res.Results))
	}

	// The valid record landed immediately — no commit step.
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM object_records WHERE organization_id=1 AND object_type_id=$1 AND deleted_at IS NULL`, typeID).Scan(&n)
	if n != 1 {
		t.Errorf("records=%d after ingest, want 1 (only the valid row)", n)
	}

	// Least privilege: an ingest-only key can reach discovery but not browse runs.
	if tg := doJSON(t, h, http.MethodGet, "/admin/imports/targets", raw, nil); tg.Code != http.StatusOK {
		t.Errorf("targets with ingest key = %d, want 200 (discovery is allowed)", tg.Code)
	}
	if list := doJSON(t, h, http.MethodGet, "/admin/imports/runs", raw, nil); list.Code != http.StatusForbidden {
		t.Errorf("runs with ingest-only key = %d, want 403 (no import.view)", list.Code)
	}
}
