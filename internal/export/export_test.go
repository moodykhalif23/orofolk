package export

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func sample() Table {
	return Table{
		Columns: []Column{{Name: "name"}, {Name: "amount", Numeric: true}},
		Rows: [][]string{
			{"Acme & Co", "100.5000"},
			{"O'Brien <Ltd>", "0"},
		},
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, sample()); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "name,amount\n") {
		t.Errorf("CSV header wrong: %q", out)
	}
	if !strings.Contains(out, "Acme & Co,100.5000") {
		t.Errorf("CSV row missing: %q", out)
	}
	// encoding/csv quotes a field containing special chars when needed.
	if !strings.Contains(out, "O'Brien <Ltd>") {
		t.Errorf("CSV second row missing: %q", out)
	}
}

func TestWriteXLSXIsValidZip(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteXLSX(&buf, sample(), "Customers"); err != nil {
		t.Fatalf("WriteXLSX: %v", err)
	}
	// .xlsx is a zip — it must open and carry the required parts.
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}
	for _, required := range []string{
		"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml",
		"xl/_rels/workbook.xml.rels", "xl/worksheets/sheet1.xml",
	} {
		if _, ok := parts[required]; !ok {
			t.Errorf("missing part %s", required)
		}
	}
	sheet := parts["xl/worksheets/sheet1.xml"]
	// Header + text cells are inline strings; the & is XML-escaped.
	if !strings.Contains(sheet, "Acme &amp; Co") {
		t.Errorf("escaped text cell missing in sheet: %s", sheet)
	}
	// A numeric column with a parseable value becomes a real numeric cell (<v>).
	if !strings.Contains(sheet, "<v>100.5000</v>") {
		t.Errorf("numeric cell missing in sheet: %s", sheet)
	}
	// The "<" in O'Brien <Ltd> must be escaped, never raw.
	if strings.Contains(sheet, "<Ltd>") {
		t.Errorf("unescaped angle bracket leaked into sheet XML")
	}
}

func TestColLetters(t *testing.T) {
	cases := map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"}
	for in, want := range cases {
		if got := colLetters(in); got != want {
			t.Errorf("colLetters(%d) = %q, want %q", in, got, want)
		}
	}
}
