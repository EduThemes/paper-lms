package models

import (
	"time"

	"gorm.io/gorm"
)

// Observer alert types (Canvas ObserverAlertThreshold alert types).
const (
	ObserverAlertAssignmentMissing   = "assignment_missing"
	ObserverAlertAssignmentGradeHigh = "assignment_grade_high"
	ObserverAlertAssignmentGradeLow  = "assignment_grade_low"
	ObserverAlertCourseGradeHigh     = "course_grade_high"
	ObserverAlertCourseGradeLow      = "course_grade_low"
	ObserverAlertCourseAnnouncement  = "course_announcement"
)

// ObserverAlertTypes lists every supported alert type, in display order.
var ObserverAlertTypes = []string{
	ObserverAlertCourseGradeLow,
	ObserverAlertCourseGradeHigh,
	ObserverAlertAssignmentGradeLow,
	ObserverAlertAssignmentGradeHigh,
	ObserverAlertAssignmentMissing,
	ObserverAlertCourseAnnouncement,
}

// ObserverAlertTypeNeedsThreshold reports whether the type compares a
// percentage against a threshold value (the grade alerts). The others are
// plain on/off switches.
func ObserverAlertTypeNeedsThreshold(alertType string) bool {
	switch alertType {
	case ObserverAlertAssignmentGradeHigh, ObserverAlertAssignmentGradeLow,
		ObserverAlertCourseGradeHigh, ObserverAlertCourseGradeLow:
		return true
	}
	return false
}

// IsObserverAlertType reports whether t is a supported alert type.
func IsObserverAlertType(t string) bool {
	for _, k := range ObserverAlertTypes {
		if k == t {
			return true
		}
	}
	return false
}

// ObserverAlertThreshold is one observer's opt-in to one alert type for one
// linked student, with the percentage threshold for the grade alerts.
type ObserverAlertThreshold struct {
	ID         uint     `json:"id" gorm:"column:id;primaryKey"`
	AccountID  uint     `json:"account_id" gorm:"not null;index"`
	ObserverID uint     `json:"observer_id" gorm:"not null;uniqueIndex:idx_observer_alert_thresholds_unique,priority:1"`
	StudentID  uint     `json:"user_id" gorm:"not null;index;uniqueIndex:idx_observer_alert_thresholds_unique,priority:2"`
	AlertType  string   `json:"alert_type" gorm:"type:varchar(40);not null;uniqueIndex:idx_observer_alert_thresholds_unique,priority:3"`
	Threshold  *float64 `json:"threshold"`
	// LastEvaluatedAt is the high-water mark of the alert scan: events
	// (grades, announcements) older than it were already considered.
	LastEvaluatedAt *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ObserverAlertWorkflow is the lifecycle of an ObserverAlert.
type ObserverAlertWorkflow string

const (
	ObserverAlertUnread    ObserverAlertWorkflow = "unread"
	ObserverAlertRead      ObserverAlertWorkflow = "read"
	ObserverAlertDismissed ObserverAlertWorkflow = "dismissed"
)

// CanTransitionTo reports whether an alert may move from s to next:
// unread -> read, and unread/read -> dismissed. Dismissed is final.
func (s ObserverAlertWorkflow) CanTransitionTo(next ObserverAlertWorkflow) bool {
	switch next {
	case ObserverAlertRead:
		return s == ObserverAlertUnread || s == ObserverAlertRead
	case ObserverAlertDismissed:
		return s == ObserverAlertUnread || s == ObserverAlertRead || s == ObserverAlertDismissed
	}
	return false
}

// ObserverAlert is one triggered alert. (ThresholdID, ContextType,
// ContextID) is unique so a rescan never raises the same alert twice.
type ObserverAlert struct {
	ID            uint                  `json:"id" gorm:"column:id;primaryKey"`
	AccountID     uint                  `json:"account_id" gorm:"not null;index"`
	ThresholdID   uint                  `json:"observer_alert_threshold_id" gorm:"not null;uniqueIndex:idx_observer_alerts_unique,priority:1"`
	ObserverID    uint                  `json:"observer_id" gorm:"not null;index"`
	StudentID     uint                  `json:"user_id" gorm:"not null;index"`
	AlertType     string                `json:"alert_type" gorm:"type:varchar(40);not null"`
	ContextType   string                `json:"context_type" gorm:"type:varchar(40);not null;uniqueIndex:idx_observer_alerts_unique,priority:2"`
	ContextID     uint                  `json:"context_id" gorm:"not null;uniqueIndex:idx_observer_alerts_unique,priority:3"`
	CourseID      *uint                 `json:"course_id"`
	Title         string                `json:"title" gorm:"type:text;not null"`
	Link          string                `json:"html_url" gorm:"type:text"`
	WorkflowState ObserverAlertWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'unread'"`
	ActionDate    time.Time             `json:"action_date"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// BeforeCreate mirrors the SQL DEFAULT 'unread' (GORM writes ” for a
// zero-value string, which would defeat the default).
func (a *ObserverAlert) BeforeCreate(_ *gorm.DB) error {
	if a.WorkflowState == "" {
		a.WorkflowState = ObserverAlertUnread
	}
	if a.ActionDate.IsZero() {
		a.ActionDate = time.Now()
	}
	return nil
}
