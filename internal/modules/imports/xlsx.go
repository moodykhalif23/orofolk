package imports

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

// parseXLSX reads the first worksheet of an .xlsx file into the same
// header-keyed rows the CSV path produces. Pure stdlib — an .xlsx is a zip of
// XML parts. It handles inline strings (what our own exporter writes), shared
// strings (what Excel / Google Sheets write) and numeric cells.
func parseXLSX(data []byte) ([]map[string]any, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("not a valid .xlsx file")
	}
	shared := readSharedStrings(zr)
	sheet := firstSheet(zr)
	if sheet == nil {
		return nil, errors.New("no worksheet found in the .xlsx file")
	}
	raw, err := readZipFile(sheet)
	if err != nil {
		return nil, err
	}
	var sh struct {
		Rows []struct {
			Cells []struct {
				R  string `xml:"r,attr"`
				T  string `xml:"t,attr"`
				V  string `xml:"v"`
				Is struct {
					T string `xml:"t"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(raw, &sh); err != nil {
		return nil, errors.New("could not parse the worksheet")
	}
	if len(sh.Rows) == 0 {
		return nil, nil
	}
	header := map[int]string{}
	for _, c := range sh.Rows[0].Cells {
		header[colIndex(c.R)] = strings.ToLower(strings.TrimSpace(cellValue(c.T, c.V, c.Is.T, shared)))
	}
	var out []map[string]any
	for _, row := range sh.Rows[1:] {
		m := map[string]any{}
		nonEmpty := false
		for _, c := range row.Cells {
			h, ok := header[colIndex(c.R)]
			if !ok || h == "" {
				continue
			}
			val := cellValue(c.T, c.V, c.Is.T, shared)
			m[h] = val
			if strings.TrimSpace(val) != "" {
				nonEmpty = true
			}
		}
		if nonEmpty {
			out = append(out, m)
		}
	}
	return out, nil
}

func readSharedStrings(zr *zip.Reader) []string {
	for _, f := range zr.File {
		if f.Name != "xl/sharedStrings.xml" {
			continue
		}
		raw, err := readZipFile(f)
		if err != nil {
			return nil
		}
		var sst struct {
			SI []struct {
				T string `xml:"t"`
				R []struct {
					T string `xml:"t"`
				} `xml:"r"`
			} `xml:"si"`
		}
		if xml.Unmarshal(raw, &sst) != nil {
			return nil
		}
		out := make([]string, len(sst.SI))
		for i, si := range sst.SI {
			if si.T != "" {
				out[i] = si.T
				continue
			}
			var b strings.Builder
			for _, r := range si.R {
				b.WriteString(r.T)
			}
			out[i] = b.String()
		}
		return out
	}
	return nil
}

func firstSheet(zr *zip.Reader) *zip.File {
	var first *zip.File
	for _, f := range zr.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			return f
		}
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			if first == nil || f.Name < first.Name {
				first = f
			}
		}
	}
	return first
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func cellValue(t, v, inlineT string, shared []string) string {
	switch t {
	case "s": // shared string: v is an index into the table
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && i >= 0 && i < len(shared) {
			return shared[i]
		}
		return ""
	case "inlineStr":
		return inlineT
	default: // "", "n" (number), "str" (formula), "b" (bool)
		return v
	}
}

// colIndex turns a cell reference's column letters ("A", "AB" from "AB12") into a
// 0-based column index.
func colIndex(ref string) int {
	n := 0
	for _, ch := range ref {
		switch {
		case ch >= 'A' && ch <= 'Z':
			n = n*26 + int(ch-'A') + 1
		case ch >= 'a' && ch <= 'z':
			n = n*26 + int(ch-'a') + 1
		default:
			return n - 1
		}
	}
	return n - 1
}
