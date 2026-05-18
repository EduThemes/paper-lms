package models

import "time"

// QuizItemBank is a course-scoped collection of reusable question templates.
// Bank items live in QuizItemBankItem and can be added to any quiz as a
// QuizQuestion that references the source item via BankItemID.
type QuizItemBank struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID        uint      `json:"course_id" gorm:"not null;index"`
	Title           string    `json:"title" gorm:"size:255;not null"`
	Description     string    `json:"description" gorm:"type:text"`
	CreatedByUserID uint      `json:"created_by_user_id" gorm:"not null;index"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (QuizItemBank) TableName() string {
	return "quiz_item_banks"
}
