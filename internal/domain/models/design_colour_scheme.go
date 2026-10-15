package models

import "time"

// DesignColourScheme is a palette a teacher saved in the Page Designer
// ("My colour schemes"). Owned by one user; AccountID is the owner's
// tenant. Palette holds the engine's Palette object (validated by
// designdoc.ValidatePalette). Migration 000137.
type DesignColourScheme struct {
	ID        uint      `json:"id" gorm:"column:id;primaryKey"`
	AccountID uint      `json:"account_id" gorm:"not null"`
	UserID    uint      `json:"user_id" gorm:"not null;index"`
	Name      string    `json:"name" gorm:"type:text;not null"`
	Palette   NullJSON  `json:"palette" gorm:"type:jsonb;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DesignColourScheme) TableName() string {
	return "design_colour_schemes"
}
