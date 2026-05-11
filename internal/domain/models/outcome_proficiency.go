package models

import "time"

// OutcomeProficiency represents a proficiency scale that defines mastery levels
// for learning outcomes. Scales can be defined at the Account level (default for
// all courses under that account) or overridden at the Course level. Modeled
// after Canvas LMS `OutcomeProficiency`.
type OutcomeProficiency struct {
	ID            uint                       `json:"id" gorm:"primaryKey"`
	ContextType   string                     `json:"context_type" gorm:"not null;index:idx_outcome_proficiency_context"` // Account | Course
	ContextID     uint                       `json:"context_id" gorm:"not null;index:idx_outcome_proficiency_context"`
	WorkflowState string                     `json:"workflow_state" gorm:"not null;default:'active'"`
	Ratings       []OutcomeProficiencyRating `json:"ratings" gorm:"foreignKey:OutcomeProficiencyID;constraint:OnDelete:CASCADE"`
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
}

func (OutcomeProficiency) TableName() string {
	return "outcome_proficiencies"
}

// OutcomeProficiencyRating is one row in a proficiency scale (e.g. "Exceeds", 4 pts).
// One rating per scale is flagged Mastery=true to signal the threshold at which a
// student is considered proficient.
type OutcomeProficiencyRating struct {
	ID                   uint      `json:"id" gorm:"primaryKey"`
	OutcomeProficiencyID uint      `json:"outcome_proficiency_id" gorm:"not null;index"`
	Description          string    `json:"description" gorm:"not null"`
	Points               float64   `json:"points" gorm:"not null"`
	Mastery              bool      `json:"mastery" gorm:"not null;default:false"`
	Color                string    `json:"color" gorm:"not null;default:'#999999'"` // hex color, e.g. "#127A1B"
	Position             int       `json:"position" gorm:"not null;default:0"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (OutcomeProficiencyRating) TableName() string {
	return "outcome_proficiency_ratings"
}

// DefaultProficiencyRatings returns the Canvas-default 4-tier scale.
// Exceeds 4, Meets (Mastery) 3, Approaching 2, Below 1.
func DefaultProficiencyRatings() []OutcomeProficiencyRating {
	return []OutcomeProficiencyRating{
		{Description: "Exceeds Mastery", Points: 4.0, Mastery: false, Color: "#02672D", Position: 1},
		{Description: "Mastery", Points: 3.0, Mastery: true, Color: "#127A1B", Position: 2},
		{Description: "Near Mastery", Points: 2.0, Mastery: false, Color: "#C66F00", Position: 3},
		{Description: "Below Mastery", Points: 1.0, Mastery: false, Color: "#E62429", Position: 4},
	}
}
