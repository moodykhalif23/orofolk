package imports

import (
	"bytes"
	"testing"

	"b2bcommerce/internal/export"
)

// TestParseXLSXRoundTrip writes a workbook with the app's own XLSX writer and
// reads it back, covering inline-string and numeric cells without a fixture.
func TestParseXLSXRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	tbl := export.Table{
		Columns: []export.Column{{Name: "sku"}, {Name: "name"}, {Name: "qty", Numeric: true}},
		Rows: [][]string{
			{"A-1", "Widget", "5"},
			{"B-2", "Gadget", "10"},
		},
	}
	if err := export.WriteXLSX(&buf, tbl, "Products"); err != nil {
		t.Fatalf("WriteXLSX: %v", err)
	}

	rows, err := parseXLSX(buf.Bytes())
	if err != nil {
		t.Fatalf("parseXLSX: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d, want 2", len(rows))
	}
	if rows[0]["sku"] != "A-1" || rows[0]["name"] != "Widget" || rows[0]["qty"] != "5" {
		t.Errorf("row 0 = %v", rows[0])
	}
	if rows[1]["sku"] != "B-2" || rows[1]["name"] != "Gadget" || rows[1]["qty"] != "10" {
		t.Errorf("row 1 = %v", rows[1])
	}
}

func TestParseXLSXRejectsNonZip(t *testing.T) {
	if _, err := parseXLSX([]byte("not a zip")); err == nil {
		t.Error("expected an error for a non-xlsx payload")
	}
}
