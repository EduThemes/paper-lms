package models

import (
	"time"

	"gorm.io/gorm"
)

// Observer link workflow states.
const (
	ObserverLinkActive  = "active"
	ObserverLinkDeleted = "deleted"
)

// ObserverLink is the durable observer → student pairing (Canvas's
// UserObservationLink, migration 000072). Before it existed the pairing
// lived only in the ObserverEnrollment rows created at link time, so it
// was a one-time snapshot (B177): a parent who paired while the child
// had no courses was refused, later courses never got an observer
// enrollment, and dropped courses stayed visible to the parent.
//
// The link is the source of truth; Enrollment.AfterSave keeps the
// observer's ObserverEnrollments in step with the student's
// StudentEnrollments for every active link.
type ObserverLink struct {
	ID         uint `json:"id" gorm:"column:id;primaryKey"`
	ObserverID uint `json:"observer_id" gorm:"not null;uniqueIndex:idx_observer_links_observer_student,priority:1"`
	StudentID  uint `json:"student_id" gorm:"not null;uniqueIndex:idx_observer_links_observer_student,priority:2;index:idx_observer_links_student_id"`
	AccountID  uint `json:"account_id" gorm:"not null;index:idx_observer_links_account_id"`
	// WorkflowState: active | deleted. NOT NULL DEFAULT 'active' in SQL;
	// mirrored in BeforeCreate (GORM writes '' for a zero string).
	WorkflowState string    `json:"workflow_state" gorm:"type:text;not null"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// BeforeCreate mirrors the SQL DEFAULT for workflow_state.
func (l *ObserverLink) BeforeCreate(tx *gorm.DB) error {
	if l.WorkflowState == "" {
		l.WorkflowState = ObserverLinkActive
	}
	return nil
}

// AfterSave propagates a StudentEnrollment's existence and state to the
// ObserverEnrollments of every observer actively linked to the student,
// so an observer follows the child into new courses and loses access to
// courses the child is dropped from. Bulk updates through
// Model(&Enrollment{}) carry a zero ID and are skipped.
func (e *Enrollment) AfterSave(tx *gorm.DB) error {
	if e.ID == 0 || e.UserID == 0 || e.CourseID == 0 || e.Type != "StudentEnrollment" {
		return nil
	}
	return SyncObserverEnrollmentsForStudent(tx.Session(&gorm.Session{NewDB: true}), e)
}

// observerMirrorState maps a student enrollment state onto the state the
// linked observer enrollment should be in. ok=false means "leave the
// observer enrollment alone" (e.g. an invitation the student has not yet
// accepted creates nothing).
func observerMirrorState(student EnrollmentWorkflow) (EnrollmentWorkflow, bool) {
	switch student {
	case EnrollmentActive, EnrollmentCompleted, EnrollmentInactive, EnrollmentDeleted:
		return student, true
	case EnrollmentRejected:
		return EnrollmentDeleted, true
	}
	return "", false
}

// SyncObserverEnrollmentsForStudent creates or updates, for every active
// ObserverLink of the student, the observer's ObserverEnrollment in the
// student enrollment's course (associated to the student) so that its
// state mirrors the student's.
func SyncObserverEnrollmentsForStudent(db *gorm.DB, student *Enrollment) error {
	target, ok := observerMirrorState(student.WorkflowState)
	if !ok {
		return nil
	}

	var links []ObserverLink
	if err := db.Where("student_id = ? AND workflow_state = ?", student.UserID, ObserverLinkActive).
		Find(&links).Error; err != nil {
		return err
	}

	for _, link := range links {
		var existing Enrollment
		err := db.Where("user_id = ? AND course_id = ? AND type = ? AND associated_user_id = ?",
			link.ObserverID, student.CourseID, "ObserverEnrollment", student.UserID).
			Order("(workflow_state = 'deleted') ASC, id ASC").
			First(&existing).Error
		switch err {
		case nil:
			if existing.WorkflowState == target {
				continue
			}
			if err := db.Model(&Enrollment{}).Where("id = ?", existing.ID).
				Update("workflow_state", target).Error; err != nil {
				return err
			}
		case gorm.ErrRecordNotFound:
			if target != EnrollmentActive {
				continue // nothing to mirror into
			}
			studentID := student.UserID
			observer := &Enrollment{
				UserID:           link.ObserverID,
				CourseID:         student.CourseID,
				CourseSectionID:  student.CourseSectionID,
				Type:             "ObserverEnrollment",
				Role:             "ObserverEnrollment",
				WorkflowState:    EnrollmentActive,
				AssociatedUserID: &studentID,
			}
			if err := db.Create(observer).Error; err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}
