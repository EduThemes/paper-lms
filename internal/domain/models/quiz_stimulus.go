package models

import "time"

// QuizStimulus is a passage (TipTap doc stored as JSONB) shared by one or more
// QuizQuestions. Each QuizQuestion may reference one stimulus via StimulusID.
type QuizStimulus struct {
	ID        uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID  uint      `json:"course_id" gorm:"not null;index"`
	Title     string    `json:"title" gorm:"size:255;not null"`
	Content   string    `json:"content" gorm:"type:jsonb"` // TipTap document JSON
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (QuizStimulus) TableName() string {
	return "quiz_stimuli"
}
