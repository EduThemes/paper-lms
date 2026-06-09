package service

import "encoding/csv"

// CSV formula injection defense.
//
// Spreadsheet apps (Excel, Sheets, LibreOffice) treat a cell whose first
// character is =, +, -, @, or a leading tab/CR as a FORMULA. A value like
// `=cmd|'/c calc'!A1` lifted from user free-text (a name, a note, an
// audit User-Agent header) then executes when the exported CSV is opened.
// Go's encoding/csv quotes for CSV grammar but does NOT neutralize this.
//
// sanitizeCSVCell prefixes an at-risk cell with a single quote, the
// canonical OWASP mitigation — the value is preserved but no longer
// parsed as a formula. Numbers and ordinary text are returned unchanged.
func sanitizeCSVCell(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	}
	return value
}

// sanitizeCSVRow applies sanitizeCSVCell to every field. Call it on each
// data row before writer.Write so no exporter has to remember which
// columns are user-controlled.
func sanitizeCSVRow(row []string) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		out[i] = sanitizeCSVCell(cell)
	}
	return out
}

// writeCSVRow writes a row with every cell formula-sanitized. Use it in
// place of writer.Write for data rows so user free-text can't smuggle a
// spreadsheet formula into an export.
func writeCSVRow(w *csv.Writer, row []string) error {
	return w.Write(sanitizeCSVRow(row))
}
