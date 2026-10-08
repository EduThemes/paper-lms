package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"log/slog"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// sisPasswordColumn is the users.csv / logins.csv column that carries a
// plaintext password.
const sisPasswordColumn = "password"

// sisBOM is a UTF-8 byte-order mark a spreadsheet may put before the
// first header.
const sisBOM = "\ufeff"

// scrubBatchPasswords blanks the password column of a batch's stored
// CSVs once the batch has run. The files are kept (diffing compares the
// next batch against them; the import page lists them), but a plaintext
// password must not stay at rest in the database (no plaintext secret
// columns). Diffing ignores the column and always re-applies a row that
// carries a password (see applyDiffing), so nothing depends on it.
func (s *SISImportService) scrubBatchPasswords(ctx context.Context, batchID uint) {
	if s.db == nil || batchID == 0 {
		return
	}
	var files []models.SISBatchFile
	if err := s.db.WithContext(ctx).Where("sis_batch_id = ?", batchID).Find(&files).Error; err != nil {
		slog.Error("sis: could not load batch files to scrub passwords", "batch_id", batchID, "error", err)
		return
	}
	for _, f := range files {
		scrubbed, changed := scrubSISPasswords(f.Content)
		if !changed {
			continue
		}
		if err := s.db.WithContext(ctx).Model(&models.SISBatchFile{}).Where("id = ?", f.ID).
			Update("content", scrubbed).Error; err != nil {
			slog.Error("sis: could not scrub passwords from a batch file", "batch_id", batchID, "file_id", f.ID, "error", err)
		}
	}
}

// scrubSISPasswords returns content with every password value emptied.
// changed=false means the CSV has no password column (or no value in
// it). A file that can't be re-read in full is cut down to its header
// rather than kept with its passwords.
func scrubSISPasswords(content []byte) ([]byte, bool) {
	r := csv.NewReader(bytes.NewReader(content))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, false
	}
	col := -1
	for i, h := range header {
		if strings.TrimSpace(strings.ToLower(strings.TrimPrefix(h, sisBOM))) == sisPasswordColumn {
			col = i
			break
		}
	}
	if col < 0 {
		return nil, false
	}
	records := [][]string{header}
	changed := false
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			records = [][]string{header}
			changed = true
			break
		}
		if col < len(rec) && rec[col] != "" {
			rec[col] = ""
			changed = true
		}
		records = append(records, rec)
	}
	if !changed {
		return nil, false
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll(records); err != nil {
		return []byte(strings.Join(header, ",") + "\n"), true
	}
	return buf.Bytes(), true
}
