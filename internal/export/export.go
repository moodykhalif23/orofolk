// Package export turns tabular data into downloadable CSV and XLSX. It is a
// pure encoder layer (no database): callers build a Table (column metadata +
// pre-rendered string rows) and stream it to an io.Writer. The data-export
// module supplies the Tables; the report builder keeps its own CSV path.
package export

import (
	"encoding/csv"
	"io"
)

type Column struct {
	Name    string
	Numeric bool
}

type Table struct {
	Columns []Column
	Rows    [][]string
}

func (t Table) header() []string {
	h := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		h[i] = c.Name
	}
	return h
}

// WriteCSV streams the table as CSV to w (RFC 4180 via encoding/csv).
func WriteCSV(w io.Writer, t Table) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(t.header()); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
