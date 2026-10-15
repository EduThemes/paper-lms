package models

import "time"

type Course struct {
	ID                uint           `json:"id" gorm:"column:id;primaryKey"`
	AccountID         uint           `json:"account_id" gorm:"not null;default:1;index"`
	Name              string         `json:"name" gorm:"not null"`
	CourseCode        string         `json:"course_code" gorm:"not null"`
	SISCourseID       *string        `json:"sis_course_id" gorm:"column:sis_course_id;uniqueIndex"`
	WorkflowState     CourseWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'available';index"`
	EnrollmentTermID  *uint          `json:"enrollment_term_id" gorm:"index"`
	StartAt           *time.Time     `json:"start_at"`
	EndAt             *time.Time     `json:"end_at"`
	DefaultView       string         `json:"default_view" gorm:"default:'modules'"`
	UIMode            string         `json:"ui_mode" gorm:"column:ui_mode;default:'standard'"` // "standard", "k2", "3-5"
	SyllabusBody      string         `json:"syllabus_body" gorm:"type:text"`
	License           string         `json:"license" gorm:"default:'private'"`
	IsPublic          bool           `json:"is_public" gorm:"default:false"`
	ApplyGroupWeights bool           `json:"apply_assignment_group_weights" gorm:"default:false"`
	NavigationTabs    string         `json:"navigation_tabs" gorm:"type:text"`
	// Date-based access settings (migration 000112; see
	// EffectiveEnrollmentAccess).
	RestrictEnrollmentsToCourseDates bool `json:"restrict_enrollments_to_course_dates" gorm:"not null;default:false"`
	RestrictStudentFutureView        bool `json:"restrict_student_future_view" gorm:"not null;default:false"`
	RestrictStudentPastView          bool `json:"restrict_student_past_view" gorm:"not null;default:false"`
	// HideFinalGrades hides the course total from students and observers
	// on the Grades page (Canvas course setting hide_final_grades).
	// Migration 000124.
	HideFinalGrades bool `json:"hide_final_grades" gorm:"not null;default:false"`
	// PostManually is the course default post policy: new assignments
	// start with a manual post policy when it is set (Canvas course
	// post_policy.post_manually).
	PostManually bool `json:"post_manually" gorm:"default:false"`
	// GradingStandardID selects the course grading scheme (a course- or
	// account-owned grading_standards row). Nil falls back to the newest
	// course scheme, then the default scale.
	GradingStandardID *uint `json:"grading_standard_id"`
	// GradingStandardDisabled turns letter grades off for course totals
	// (Canvas "Enable course grading scheme" unchecked).
	GradingStandardDisabled bool `json:"grading_standard_disabled" gorm:"default:false"`
	// Look is the course header look from the Page Designer (theme,
	// colours, banner); NULL keeps Paper's generated header. Validated by
	// designdoc.ValidateLook. Migration 000136.
	Look      NullJSON  `json:"look" gorm:"column:look;type:jsonb"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
