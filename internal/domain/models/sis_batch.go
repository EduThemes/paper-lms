package models

import "time"

type SISBatch struct {
	ID            uint   `json:"id" gorm:"column:id;primaryKey"`
	AccountID     uint   `json:"account_id" gorm:"not null;index"`
	WorkflowState string `json:"workflow_state" gorm:"not null;default:'created'"` // created, importing, imported, imported_with_messages, failed, aborted
	Progress      int    `json:"progress" gorm:"default:0"`                        // 0-100
	Data          string `json:"data" gorm:"type:text"`                            // JSON metadata about the import (per-file counts)
	TotalRows     int    `json:"total_rows" gorm:"default:0"`
	ProcessedRows int    `json:"processed_rows" gorm:"default:0"`

	// G111 — import options (migration 000117).
	//
	// UserID is the admin who started the import. BatchMode + the term
	// make every SIS-managed course, section and enrollment of that term
	// that the batch does not mention deleted (Canvas batch mode).
	// DiffingDataSetIdentifier makes the import apply only the rows that
	// changed since the last batch with the same identifier; rows that
	// disappeared are treated as deleted. ChangeThreshold (percent) aborts
	// batch-mode / diffing deletes that would remove more than that share.
	UserID                   uint       `json:"user_id" gorm:"not null;default:0"`
	BatchMode                bool       `json:"batch_mode" gorm:"not null;default:false"`
	BatchModeTermID          *uint      `json:"batch_mode_term_id" gorm:"column:batch_mode_term_id"`
	DiffingDataSetIdentifier *string    `json:"diffing_data_set_identifier" gorm:"column:diffing_data_set_identifier;index"`
	DiffedAgainstBatchID     *uint      `json:"diffed_against_sis_batch_id" gorm:"column:diffed_against_batch_id"`
	ChangeThreshold          *int       `json:"change_threshold" gorm:"column:change_threshold"`
	StartedAt                *time.Time `json:"started_at"`
	EndedAt                  *time.Time `json:"ended_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SISBatchError struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey"`
	SISBatchID uint      `json:"sis_batch_id" gorm:"column:sis_batch_id;not null;index"`
	Row        int       `json:"row"`
	Message    string    `json:"message" gorm:"type:text;not null"`
	File       string    `json:"file"`
	CreatedAt  time.Time `json:"created_at"`
}

// SISBatchFile is one CSV of a SIS batch (G111). The upload is kept so the
// batch can be processed off the request path and so a later batch with
// the same diffing identifier can be diffed against it.
type SISBatchFile struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey"`
	SISBatchID uint      `json:"sis_batch_id" gorm:"column:sis_batch_id;not null;index"`
	FileName   string    `json:"file_name" gorm:"not null"`
	ImportType string    `json:"import_type" gorm:"not null"` // users, courses, ... (detected from the header)
	Content    []byte    `json:"-" gorm:"type:bytea;not null"`
	CreatedAt  time.Time `json:"created_at"`
}
