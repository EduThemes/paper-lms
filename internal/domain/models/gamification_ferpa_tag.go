package models

import "time"

// GamificationFerpaFieldTag maps an event-shape (ObjectType, FieldPath)
// to a FERPA classification. The FERPA guard (Wave 1 task 11, later PR)
// consults this on every Emit so that, e.g., a raw score in a non-PII
// flagged context is rejected before the row hits the event store.
//
// FieldPath is a JSONPath into the event's result/context JSONB.
// Classification is one of:
//
//	directory_information | education_record | non_PII | instructor_metadata
//
// Composite PK (object_type, field_path) enforced by the SQL chain.
type GamificationFerpaFieldTag struct {
	ObjectType     string    `json:"object_type" gorm:"primaryKey"`
	FieldPath      string    `json:"field_path" gorm:"primaryKey"`
	Classification string    `json:"classification" gorm:"not null"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null;default:now()"`
}

func (GamificationFerpaFieldTag) TableName() string { return "gamification_ferpa_field_tags" }
