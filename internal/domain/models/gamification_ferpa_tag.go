package models

import "time"

// FerpaClassification is the typed string for the four FERPA buckets
// the gamification surface uses to gate visibility. The DB column is
// `text` with a CHECK constraint (migration 000034) limiting values to
// the same four constants; this Go-side type adds compile-time
// enforcement so a `currency.FerpaClassification = "PII"` typo can't
// ship past the type checker.
//
// Closing F1.7 from docs/audits/2026-05-15-gamification-audit.md.
type FerpaClassification string

const (
	// FerpaDirectoryInformation — name, grade, address: shareable
	// per default unless the parent has opted out under FERPA §99.37.
	FerpaDirectoryInformation FerpaClassification = "directory_information"

	// FerpaEducationRecord — protected under FERPA §99.3. Cannot
	// appear on public surfaces (leaderboards, parent dashboards
	// other than the student's own parent) without consent.
	FerpaEducationRecord FerpaClassification = "education_record"

	// FerpaNonPII — gameplay-only counters with no identifying value
	// (XP, gems, generic badge counts).
	FerpaNonPII FerpaClassification = "non_PII"

	// FerpaInstructorMetadata — telemetry visible only to instructors
	// (effort scores, grading-pace counters). Never visible to
	// students or parents.
	FerpaInstructorMetadata FerpaClassification = "instructor_metadata"
)

// IsValid reports whether the value is one of the four enum constants.
// The DB CHECK is the authoritative guard; this is the early-rejection
// helper for handler-side validation.
func (f FerpaClassification) IsValid() bool {
	switch f {
	case FerpaDirectoryInformation, FerpaEducationRecord, FerpaNonPII, FerpaInstructorMetadata:
		return true
	}
	return false
}

// String implements fmt.Stringer and lets the typed value drop into
// any format-string call site that previously consumed a bare string.
func (f FerpaClassification) String() string { return string(f) }

// GamificationFerpaFieldTag maps an event-shape (ObjectType, FieldPath)
// to a FERPA classification. The FERPA guard (Wave 1 task 11, later PR)
// consults this on every Emit so that, e.g., a raw score in a non-PII
// flagged context is rejected before the row hits the event store.
//
// FieldPath is a JSONPath into the event's result/context JSONB.
//
// Composite PK (object_type, field_path) enforced by the SQL chain.
type GamificationFerpaFieldTag struct {
	ObjectType     string              `json:"object_type" gorm:"primaryKey"`
	FieldPath      string              `json:"field_path" gorm:"primaryKey"`
	Classification FerpaClassification `json:"classification" gorm:"not null;type:text"`
	Description    string              `json:"description"`
	CreatedAt      time.Time           `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt      time.Time           `json:"updated_at" gorm:"not null;default:now()"`
}

func (GamificationFerpaFieldTag) TableName() string { return "gamification_ferpa_field_tags" }
