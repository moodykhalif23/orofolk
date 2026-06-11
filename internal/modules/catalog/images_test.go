package catalog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// maxProductImagesTest mirrors the handler's unexported cap (this is an
// external test package). Keep in sync with catalog.maxProductImages.
const maxProductImagesTest = 5

// seedMediaAsset inserts a ready media asset for an org and returns its id.
func seedMediaAsset(t *testing.T, pool *pgxpool.Pool, org int64, url string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO media_assets (organization_id, url, mime_type, status) VALUES ($1, $2, 'image/jpeg', 'ready') RETURNING id`,
		org, url).Scan(&id); err != nil {
		t.Fatalf("seed media asset: %v", err)
	}
	return id
}

// createAdminProduct creates a product via the admin API and returns its id.
func createAdminProduct(t *testing.T, h http.Handler, token, sku, slug string) int64 {
	t.Helper()
	rr := do(t, h, http.MethodPost, "/admin/products", token, map[string]any{"sku": sku, "name": sku, "slug": slug})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create product %s: want 201, got %d (%s)", sku, rr.Code, rr.Body.String())
	}
	var p struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &p)
	if p.ID == 0 {
		t.Fatalf("create product %s: no id in response (%s)", sku, rr.Body.String())
	}
	return p.ID
}

// doUpload posts a single-file multipart/form-data request (field "file").
func doUpload(t *testing.T, h http.Handler, path, token, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

type imageListResp struct {
	Items []struct {
		ID  int64  `json:"id"`
		URL string `json:"url"`
	} `json:"items"`
}

func TestProductImages_AddListDeleteAndMaxFive(t *testing.T) {
	h, issuer, pool := newServer(t)
	tok := catalogToken(t, issuer)
	pid := createAdminProduct(t, h, tok, "IMG-MAX", "img-max")
	base := fmt.Sprintf("/admin/products/%d/images", pid)

	var firstImageID int64
	for i := 0; i < maxProductImagesTest; i++ {
		aid := seedMediaAsset(t, pool, 1, fmt.Sprintf("/media/file/a%d.jpg", i))
		rr := do(t, h, http.MethodPost, base, tok, map[string]any{"media_asset_id": aid})
		if rr.Code != http.StatusCreated {
			t.Fatalf("add image %d: want 201, got %d (%s)", i, rr.Code, rr.Body.String())
		}
		if i == 0 {
			var im struct {
				ID  int64  `json:"id"`
				URL string `json:"url"`
			}
			_ = json.Unmarshal(rr.Body.Bytes(), &im)
			firstImageID = im.ID
			if im.URL != "/media/file/a0.jpg" {
				t.Errorf("image url denormalized wrong: got %q", im.URL)
			}
		}
	}

	// The 6th image is rejected — the cap holds.
	aidOver := seedMediaAsset(t, pool, 1, "/media/file/over.jpg")
	rr := do(t, h, http.MethodPost, base, tok, map[string]any{"media_asset_id": aidOver})
	if rr.Code != http.StatusConflict {
		t.Fatalf("6th image: want 409, got %d (%s)", rr.Code, rr.Body.String())
	}

	// List returns exactly the cap.
	lr := do(t, h, http.MethodGet, base, tok, nil)
	var list imageListResp
	_ = json.Unmarshal(lr.Body.Bytes(), &list)
	if len(list.Items) != maxProductImagesTest {
		t.Fatalf("list images: want %d, got %d", maxProductImagesTest, len(list.Items))
	}

	// Delete one, then a new one can be added (cap is a live count, not a high-water mark).
	dr := do(t, h, http.MethodDelete, fmt.Sprintf("%s/%d", base, firstImageID), tok, nil)
	if dr.Code != http.StatusNoContent {
		t.Fatalf("delete image: want 204, got %d (%s)", dr.Code, dr.Body.String())
	}
	rr = do(t, h, http.MethodPost, base, tok, map[string]any{"media_asset_id": aidOver})
	if rr.Code != http.StatusCreated {
		t.Fatalf("add after delete: want 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	lr = do(t, h, http.MethodGet, base, tok, nil)
	_ = json.Unmarshal(lr.Body.Bytes(), &list)
	if len(list.Items) != maxProductImagesTest {
		t.Fatalf("list after re-add: want %d, got %d", maxProductImagesTest, len(list.Items))
	}
}

func TestProductImages_RejectsUnknownAsset(t *testing.T) {
	h, issuer, _ := newServer(t)
	tok := catalogToken(t, issuer)
	pid := createAdminProduct(t, h, tok, "IMG-ASSET", "img-asset")
	// A media_asset id that does not exist in this org is rejected (org-scoped lookup).
	rr := do(t, h, http.MethodPost, fmt.Sprintf("/admin/products/%d/images", pid), tok, map[string]any{"media_asset_id": 999999})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown asset: want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestProductImages_UnknownProduct404(t *testing.T) {
	h, issuer, _ := newServer(t)
	tok := catalogToken(t, issuer)
	rr := do(t, h, http.MethodGet, "/admin/products/999999/images", tok, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown product images: want 404, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestProductCSV_ImportThenUpdateThenExport(t *testing.T) {
	h, issuer, _ := newServer(t)
	tok := catalogToken(t, issuer)

	// Import: one good row, one missing-sku row (must be reported, not fatal).
	csvIn := "sku,name,slug,status\nBULK-1,Bulk One,bulk-1,active\n,No SKU,bad-row,active\n"
	rr := doUpload(t, h, "/admin/products/import", tok, "products.csv", []byte(csvIn))
	if rr.Code != http.StatusOK {
		t.Fatalf("import: want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var res struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
		Errors  int `json:"errors"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.Created != 1 || res.Updated != 0 || res.Errors != 1 {
		t.Fatalf("import counts: want created=1 updated=0 errors=1, got %+v (%s)", res, rr.Body.String())
	}

	// Re-import the same SKU with a new name → update by SKU, no duplicate.
	csvUpd := "sku,name,slug,status\nBULK-1,Bulk One Updated,bulk-1,active\n"
	rr = doUpload(t, h, "/admin/products/import", tok, "products.csv", []byte(csvUpd))
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res.Created != 0 || res.Updated != 1 {
		t.Fatalf("re-import counts: want created=0 updated=1, got %+v (%s)", res, rr.Body.String())
	}

	// Export: must route to the CSV exporter (not /admin/products/{id}) and
	// contain the updated product.
	er := do(t, h, http.MethodGet, "/admin/products/export", tok, nil)
	if er.Code != http.StatusOK {
		t.Fatalf("export: want 200, got %d (%s)", er.Code, er.Body.String())
	}
	if ct := er.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("export content-type: want text/csv, got %q", ct)
	}
	body := er.Body.String()
	if !strings.Contains(body, "BULK-1") || !strings.Contains(body, "Bulk One Updated") {
		t.Errorf("export body missing updated product:\n%s", body)
	}
}
