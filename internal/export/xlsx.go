package export

import (
	"archive/zip"
	"io"
	"strconv"
	"strings"
)

// WriteXLSX writes a minimal, valid single-sheet .xlsx workbook to w using only
// the standard library (an .xlsx is a zip of a few XML parts). Cells are written
// as inline strings, except columns marked Numeric whose value parses as a number
// — those become real numeric cells so Excel can sum them. This is deliberately
// minimal (one sheet, no styles/shared-strings); it is sufficient for tabular
// data exports and avoids a third-party dependency.
func WriteXLSX(w io.Writer, t Table, sheetName string) error {
	zw := zip.NewWriter(w)
	parts := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypesXML},
		{"_rels/.rels", rootRelsXML},
		{"xl/workbook.xml", workbookXML(sheetName)},
		{"xl/_rels/workbook.xml.rels", workbookRelsXML},
		{"xl/worksheets/sheet1.xml", sheetXML(t)},
	}
	for _, p := range parts {
		f, err := zw.Create(p.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(f, p.body); err != nil {
			return err
		}
	}
	return zw.Close()
}

const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`

const rootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const workbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`

func workbookXML(sheetName string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="` + escapeAttr(sheetTitle(sheetName)) + `" sheetId="1" r:id="rId1"/></sheets>
</workbook>`
}

func sheetXML(t Table) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)

	// Header row (row 1) — always inline strings.
	b.WriteString(`<row r="1">`)
	for i, c := range t.Columns {
		writeStringCell(&b, cellRef(i, 1), c.Name)
	}
	b.WriteString(`</row>`)

	// Data rows.
	for r, row := range t.Rows {
		rowNum := r + 2
		b.WriteString(`<row r="`)
		b.WriteString(strconv.Itoa(rowNum))
		b.WriteString(`">`)
		for i, val := range row {
			ref := cellRef(i, rowNum)
			if i < len(t.Columns) && t.Columns[i].Numeric && isNumber(val) {
				b.WriteString(`<c r="`)
				b.WriteString(ref)
				b.WriteString(`"><v>`)
				b.WriteString(val)
				b.WriteString(`</v></c>`)
			} else {
				writeStringCell(&b, ref, val)
			}
		}
		b.WriteString(`</row>`)
	}

	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func writeStringCell(b *strings.Builder, ref, val string) {
	b.WriteString(`<c r="`)
	b.WriteString(ref)
	b.WriteString(`" t="inlineStr"><is><t xml:space="preserve">`)
	b.WriteString(escapeText(val))
	b.WriteString(`</t></is></c>`)
}

// cellRef builds an A1-style reference (col is 0-based, row is 1-based).
func cellRef(col, row int) string { return colLetters(col) + strconv.Itoa(row) }

func colLetters(col int) string {
	var s string
	col++ // 1-based
	for col > 0 {
		col--
		s = string(rune('A'+col%26)) + s
		col /= 26
	}
	return s
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// sheetTitle sanitises a worksheet name (≤31 chars, no \ / ? * [ ] :).
func sheetTitle(name string) string {
	if name == "" {
		name = "Sheet1"
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', '?', '*', '[', ']', ':':
			return ' '
		default:
			return r
		}
	}, name)
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

func escapeText(s string) string {
	return textReplacer.Replace(s)
}

func escapeAttr(s string) string {
	return attrReplacer.Replace(s)
}

var textReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

var attrReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
