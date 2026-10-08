package postgres

import (
	"context"
	"sort"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// usableInCourseSQL is the visibility rule for a bank from a course: the
// course's own active banks, plus active banks another course in the SAME
// account shared with the account. Never crosses tenants.
const usableInCourseSQL = `workflow_state = 'active' AND (
	course_id = ?
	OR (shared_with_account = true
	    AND COALESCE(account_id, (SELECT oc.account_id FROM courses oc WHERE oc.id = question_banks.course_id))
	        = (SELECT rc.account_id FROM courses rc WHERE rc.id = ?))
)`

// ListUsableByCourse returns the course's own banks followed by the banks
// shared with its account.
func (r *questionBankRepo) ListUsableByCourse(ctx context.Context, courseID uint) ([]models.QuestionBank, error) {
	var banks []models.QuestionBank
	if err := r.db.WithContext(ctx).
		Where(usableInCourseSQL, courseID, courseID).
		Order("id ASC").
		Find(&banks).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(banks, func(i, j int) bool {
		return banks[i].CourseID == courseID && banks[j].CourseID != courseID
	})
	return banks, nil
}

// UsableInCourse reports whether bankID may be read / drawn from by
// courseID (own or account-shared, active).
func (r *questionBankRepo) UsableInCourse(ctx context.Context, bankID, courseID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.QuestionBank{}).
		Where("id = ?", bankID).
		Where(usableInCourseSQL, courseID, courseID).
		Count(&n).Error
	return n > 0, err
}
