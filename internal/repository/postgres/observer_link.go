package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// ObserverLinkRepository persists observer -> student pairings
// (migration 000072, B177).
type ObserverLinkRepository struct {
	db *gorm.DB
}

func NewObserverLinkRepository(db *gorm.DB) *ObserverLinkRepository {
	return &ObserverLinkRepository{db: db}
}

// Activate creates the link, or reactivates a previously removed one.
// created reports whether the link was not already active.
func (r *ObserverLinkRepository) Activate(ctx context.Context, observerID, studentID, accountID uint) (bool, error) {
	db := r.db.WithContext(ctx)
	var link models.ObserverLink
	err := db.Where("observer_id = ? AND student_id = ?", observerID, studentID).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		link = models.ObserverLink{ObserverID: observerID, StudentID: studentID, AccountID: accountID, WorkflowState: models.ObserverLinkActive}
		return true, db.Create(&link).Error
	}
	if err != nil {
		return false, err
	}
	if link.WorkflowState == models.ObserverLinkActive {
		return false, nil
	}
	return true, db.Model(&models.ObserverLink{}).Where("id = ?", link.ID).
		Updates(map[string]interface{}{"workflow_state": models.ObserverLinkActive, "account_id": accountID}).Error
}

// Deactivate removes an active link. removed reports whether one existed.
func (r *ObserverLinkRepository) Deactivate(ctx context.Context, observerID, studentID uint) (bool, error) {
	res := r.db.WithContext(ctx).Model(&models.ObserverLink{}).
		Where("observer_id = ? AND student_id = ? AND workflow_state = ?", observerID, studentID, models.ObserverLinkActive).
		Update("workflow_state", models.ObserverLinkDeleted)
	return res.RowsAffected > 0, res.Error
}

// IsLinked reports whether an active link exists.
func (r *ObserverLinkRepository) IsLinked(ctx context.Context, observerID, studentID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.ObserverLink{}).
		Where("observer_id = ? AND student_id = ? AND workflow_state = ?", observerID, studentID, models.ObserverLinkActive).
		Count(&n).Error
	return n > 0, err
}

// ListStudentIDs returns the students an observer is actively linked to.
func (r *ObserverLinkRepository) ListStudentIDs(ctx context.Context, observerID uint) ([]uint, error) {
	var ids []uint
	err := r.db.WithContext(ctx).Model(&models.ObserverLink{}).
		Where("observer_id = ? AND workflow_state = ?", observerID, models.ObserverLinkActive).
		Order("student_id ASC").
		Pluck("student_id", &ids).Error
	return ids, err
}

// SyncStudent mirrors every StudentEnrollment of the student onto the
// ObserverEnrollments of the student's actively linked observers. Used
// right after a link is created so the observer immediately sees the
// child's current courses; later enrollment changes are mirrored by
// Enrollment.AfterSave.
func (r *ObserverLinkRepository) SyncStudent(ctx context.Context, studentID uint) error {
	db := r.db.WithContext(ctx)
	var enrollments []models.Enrollment
	if err := db.Where("user_id = ? AND type = ?", studentID, "StudentEnrollment").
		// Active rows last, so a stale duplicate never wins over them.
		Order("(workflow_state = 'active') ASC, id ASC").Find(&enrollments).Error; err != nil {
		return err
	}
	for i := range enrollments {
		if err := models.SyncObserverEnrollmentsForStudent(db, &enrollments[i]); err != nil {
			return err
		}
	}
	return nil
}
