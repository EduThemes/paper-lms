package models

import "time"

type Assignment struct {
	ID                 uint               `json:"id" gorm:"column:id;primaryKey"`
	CourseID           uint               `json:"course_id" gorm:"not null;index"`
	Name               string             `json:"name" gorm:"not null"`
	Description        string             `json:"description" gorm:"type:text"`
	DueAt              *time.Time         `json:"due_at"`
	UnlockAt           *time.Time         `json:"unlock_at"`
	LockAt             *time.Time         `json:"lock_at"`
	PointsPossible     *float64           `json:"points_possible"`
	GradingType        string             `json:"grading_type" gorm:"default:'points'"`                // points, percent, letter_grade, gpa_scale, pass_fail, not_graded
	SubmissionTypes    string             `json:"submission_types" gorm:"default:'online_text_entry'"` // comma-separated
	AssignmentGroupID  *uint              `json:"assignment_group_id" gorm:"index"`
	Position           int                `json:"position"`
	WorkflowState      AssignmentWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'unpublished';index"`
	Published          bool               `json:"published" gorm:"default:false"`
	AnonymousGrading   bool               `json:"anonymous_grading" gorm:"default:false"`
	PostPolicy         string             `json:"post_policy" gorm:"default:'automatic'"` // automatic, manual
	PeerReviewsEnabled bool               `json:"peer_reviews_enabled" gorm:"default:false"`
	PeerReviewCount    int                `json:"peer_review_count" gorm:"default:0"`
	GroupCategoryID    *uint              `json:"group_category_id" gorm:"index"`
	// OnlyVisibleToOverrides limits the assignment to the students and
	// sections that have an override ("Assign to" without Everyone).
	OnlyVisibleToOverrides bool `json:"only_visible_to_overrides" gorm:"default:false"`
	// GradingStandardID is the per-assignment scheme for letter_grade /
	// gpa_scale assignments; nil uses the course scheme.
	GradingStandardID *uint `json:"grading_standard_id"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`

	// Submission settings, group grading, moderated grading and peer
	// review options (grading-ui lane, migration 000093). Canvas names.
	// Every bool defaults to false in the DB (GORM omits false on
	// insert); handlers set the Canvas "default true" values explicitly.
	AllowedAttempts                 *int       `json:"allowed_attempts"`                    // nil or <= 0: unlimited
	AllowedExtensions               string     `json:"allowed_extensions" gorm:"type:text"` // comma-separated, lower-case, no dots
	AnnotatableAttachmentID         *uint      `json:"annotatable_attachment_id" gorm:"index"`
	GradeGroupStudentsIndividually  bool       `json:"grade_group_students_individually" gorm:"default:false"`
	ModeratedGrading                bool       `json:"moderated_grading" gorm:"default:false"`
	GraderCount                     int        `json:"grader_count" gorm:"default:0"`
	FinalGraderID                   *uint      `json:"final_grader_id"`
	GraderCommentsVisibleToGraders  bool       `json:"grader_comments_visible_to_graders" gorm:"default:false"`
	GradersAnonymousToGraders       bool       `json:"graders_anonymous_to_graders" gorm:"default:false"`
	GraderNamesVisibleToFinalGrader bool       `json:"grader_names_visible_to_final_grader" gorm:"default:false"`
	GradesPublishedAt               *time.Time `json:"grades_published_at"`
	AnonymousPeerReviews            bool       `json:"anonymous_peer_reviews" gorm:"default:false"`
	PeerReviewsDueAt                *time.Time `json:"peer_reviews_due_at"`

	// OmitFromFinalGrade keeps a graded assignment out of the course
	// total (Canvas omit_from_final_grade; migration 000097).
	OmitFromFinalGrade bool `json:"omit_from_final_grade" gorm:"default:false"`
}

// IsPublished reports whether students may see and submit to the
// assignment. workflow_state is the source of truth; the legacy
// Published bool is honored for rows written before the state machine
// kept both in sync. A deleted assignment is never published.
func (a *Assignment) IsPublished() bool {
	if a == nil || a.WorkflowState == AssignmentDeleted {
		return false
	}
	if a.WorkflowState == AssignmentPublished {
		return true
	}
	return a.WorkflowState == "" && a.Published
}

// UsesManualPostPolicy reports whether grades stay hidden from students
// until an instructor explicitly posts them.
func (a *Assignment) UsesManualPostPolicy() bool {
	return a != nil && a.PostPolicy == "manual"
}

// GradesPublished mirrors Canvas Assignment#grades_published?: a
// moderated assignment's grades exist only once the moderator releases
// them; any other assignment is always "published".
func (a *Assignment) GradesPublished() bool {
	return a != nil && (!a.ModeratedGrading || a.GradesPublishedAt != nil)
}

// IsGroupAssignment reports whether the assignment is submitted by groups.
func (a *Assignment) IsGroupAssignment() bool {
	return a != nil && a.GroupCategoryID != nil && *a.GroupCategoryID > 0
}

// GradesGroupTogether reports whether one grade is copied to every
// member of a group (Canvas default) rather than graded per student.
func (a *Assignment) GradesGroupTogether() bool {
	return a.IsGroupAssignment() && !a.GradeGroupStudentsIndividually
}

// AttemptsLimit returns the allowed attempts, 0 meaning unlimited.
func (a *Assignment) AttemptsLimit() int {
	if a == nil || a.AllowedAttempts == nil || *a.AllowedAttempts <= 0 {
		return 0
	}
	return *a.AllowedAttempts
}
