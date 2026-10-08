package models

import "time"

// PlannerNote - personal reminder notes created by students
type PlannerNote struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	UserID        uint      `json:"user_id" gorm:"not null;index"`
	Title         string    `json:"title" gorm:"type:varchar(255);not null"`
	Details       string    `json:"details" gorm:"type:text"`
	TodoDate      time.Time `json:"todo_date" gorm:"index"`
	CourseID      *uint     `json:"course_id,omitempty" gorm:"index"`
	WorkflowState string    `json:"workflow_state" gorm:"type:varchar(50);default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PlannerOverride - allows students to mark items as done or dismissed.
// One row per (user, plannable); migration 000163 backs the unique index.
type PlannerOverride struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	UserID         uint      `json:"user_id" gorm:"not null;index;uniqueIndex:idx_planner_overrides_user_plannable,priority:1"`
	PlannableType  string    `json:"plannable_type" gorm:"type:varchar(50);not null;index;uniqueIndex:idx_planner_overrides_user_plannable,priority:2"` // "assignment", "quiz", "discussion_topic", "wiki_page", "planner_note", "calendar_event", "announcement"
	PlannableID    uint      `json:"plannable_id" gorm:"not null;index;uniqueIndex:idx_planner_overrides_user_plannable,priority:3"`
	MarkedComplete bool      `json:"marked_complete" gorm:"default:false"`
	Dismissed      bool      `json:"dismissed" gorm:"default:false"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// UserCoursePreference is one user's dashboard customization of one
// course (Canvas: favorites, course nicknames and custom colors). A row
// only exists once the user changed something; no row means "not
// favorited, no nickname, default color". Nickname and Color are
// pointers so "unset" is SQL NULL rather than an empty string.
type UserCoursePreference struct {
	ID       uint    `json:"id" gorm:"column:id;primaryKey"`
	UserID   uint    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_course_prefs_user_course,priority:1"`
	CourseID uint    `json:"course_id" gorm:"not null;index;uniqueIndex:idx_user_course_prefs_user_course,priority:2"`
	Favorite bool    `json:"favorite" gorm:"not null;default:false"`
	Nickname *string `json:"nickname" gorm:"type:varchar(60)"`
	Color    *string `json:"color" gorm:"type:varchar(7)"`
	// Position orders favorited cards on the dashboard (lower first).
	Position  *int      `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
