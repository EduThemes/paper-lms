package models

import "time"

// CustomGradebookColumn is a Canvas-compatible custom gradebook column
// (e.g., notes, free-form text columns shown alongside assignment columns).
// See: canvas-lms/app/models/custom_gradebook_column.rb
type CustomGradebookColumn struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CourseID      uint      `gorm:"not null;index:idx_custom_gradebook_columns_course_pos" json:"course_id"`
	Title         string    `gorm:"type:varchar(255);not null" json:"title"`
	Position      int       `gorm:"not null;default:0;index:idx_custom_gradebook_columns_course_pos" json:"position"`
	Hidden        bool      `gorm:"not null;default:false" json:"hidden"`
	ReadOnly      bool      `gorm:"not null;default:false" json:"read_only"`
	TeacherNotes  bool      `gorm:"not null;default:false" json:"teacher_notes"`
	WorkflowState string    `gorm:"type:varchar(32);not null;default:'active';index" json:"workflow_state"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName overrides the default GORM pluralization to match Canvas.
func (CustomGradebookColumn) TableName() string {
	return "custom_gradebook_columns"
}

// CustomColumnDatum stores the per-student value for a custom gradebook column.
// Canvas stores Content as a string up to ~4KB.
type CustomColumnDatum struct {
	ID                      uint      `gorm:"primaryKey" json:"id"`
	CustomGradebookColumnID uint      `gorm:"not null;uniqueIndex:idx_custom_column_data_col_user" json:"custom_gradebook_column_id"`
	UserID                  uint      `gorm:"not null;uniqueIndex:idx_custom_column_data_col_user;index" json:"user_id"`
	Content                 string    `gorm:"type:text" json:"content"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (CustomColumnDatum) TableName() string {
	return "custom_gradebook_column_data"
}
