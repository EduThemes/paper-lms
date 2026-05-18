package models

import "time"

type SISBatch struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID     uint      `json:"account_id" gorm:"not null;index"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'created'"` // created, importing, imported, imported_with_messages, failed
	Progress      int       `json:"progress" gorm:"default:0"`                        // 0-100
	Data          string    `json:"data" gorm:"type:text"`                            // JSON metadata about the import
	TotalRows     int       `json:"total_rows" gorm:"default:0"`
	ProcessedRows int       `json:"processed_rows" gorm:"default:0"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SISBatchError struct {
	ID         uint      `json:"id" gorm:"column:id;primaryKey"`
	SISBatchID uint      `json:"sis_batch_id" gorm:"column:sis_batch_id;not null;index"`
	Row        int       `json:"row"`
	Message    string    `json:"message" gorm:"type:text;not null"`
	File       string    `json:"file"`
	CreatedAt  time.Time `json:"created_at"`
}
