package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// submissionTenantScope gates submission-keyed rows (comments) through
// submission -> assignment -> course -> account.
const submissionTenantScope = "submission_id IN (SELECT id FROM submissions WHERE assignment_id IN (SELECT id FROM assignments WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))"

type gradingUIRepo struct {
	db *gorm.DB
}

// NewGradingUIRepository returns the Postgres GradingUIRepository.
func NewGradingUIRepository(db *gorm.DB) repository.GradingUIRepository {
	return &gradingUIRepo{db: db}
}

// --- submission attempt history ---------------------------------------

func (r *gradingUIRepo) CreateSubmissionVersion(ctx context.Context, v *models.SubmissionVersion) error {
	// A resubmission that reuses an attempt number (it should not happen,
	// but a racing double-click could) replaces the earlier snapshot.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("submission_id = ? AND attempt = ?", v.SubmissionID, v.Attempt).
			Delete(&models.SubmissionVersion{}).Error; err != nil {
			return err
		}
		return tx.Create(v).Error
	})
}

func (r *gradingUIRepo) ListSubmissionVersions(ctx context.Context, submissionID uint) ([]models.SubmissionVersion, error) {
	var out []models.SubmissionVersion
	err := r.db.WithContext(ctx).Where("submission_id = ?", submissionID).Order("attempt ASC").Find(&out).Error
	return out, err
}

func (r *gradingUIRepo) ListSubmissionVersionsByAssignment(ctx context.Context, assignmentID uint) ([]models.SubmissionVersion, error) {
	var out []models.SubmissionVersion
	err := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).Order("submission_id ASC, attempt ASC").Find(&out).Error
	return out, err
}

// --- submissions -------------------------------------------------------

func (r *gradingUIRepo) ListSubmissionsByAssignment(ctx context.Context, assignmentID uint) ([]models.Submission, error) {
	var out []models.Submission
	err := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingUIRepo) FindSubmissionByAnonymousID(ctx context.Context, assignmentID uint, anonymousID string, accountID uint) (*models.Submission, error) {
	var sub models.Submission
	q := r.db.WithContext(ctx).Where("assignment_id = ? AND anonymous_id = ?", assignmentID, anonymousID)
	if accountID != 0 {
		q = q.Where(assignmentTenantScope, accountID)
	}
	if err := q.First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *gradingUIRepo) UpdateSubmissionColumns(ctx context.Context, submissionID uint, cols map[string]interface{}) error {
	cols["updated_at"] = time.Now()
	return r.db.WithContext(ctx).Model(&models.Submission{}).Where("id = ?", submissionID).Updates(cols).Error
}

// --- submission comments -----------------------------------------------

func (r *gradingUIRepo) FindSubmissionComment(ctx context.Context, id, accountID uint) (*models.SubmissionComment, error) {
	var c models.SubmissionComment
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where(submissionTenantScope, accountID)
	}
	if err := q.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *gradingUIRepo) UpdateSubmissionComment(ctx context.Context, c *models.SubmissionComment) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *gradingUIRepo) DeleteSubmissionComment(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.SubmissionComment{}, id).Error
}

func (r *gradingUIRepo) ListSubmissionCommentsByGroupCommentID(ctx context.Context, groupCommentID string) ([]models.SubmissionComment, error) {
	var out []models.SubmissionComment
	err := r.db.WithContext(ctx).Where("group_comment_id = ?", groupCommentID).Order("id ASC").Find(&out).Error
	return out, err
}

// --- moderated grading -------------------------------------------------

func (r *gradingUIRepo) FindModerationGrader(ctx context.Context, assignmentID, userID uint) (*models.ModerationGrader, error) {
	var g models.ModerationGrader
	if err := r.db.WithContext(ctx).Where("assignment_id = ? AND user_id = ?", assignmentID, userID).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *gradingUIRepo) ListModerationGraders(ctx context.Context, assignmentID uint) ([]models.ModerationGrader, error) {
	var out []models.ModerationGrader
	err := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).Order("anonymous_id ASC").Find(&out).Error
	return out, err
}

func (r *gradingUIRepo) SaveModerationGrader(ctx context.Context, g *models.ModerationGrader) error {
	// Save (not Create) so slot_taken=false is written, not left to the
	// column default.
	return r.db.WithContext(ctx).Save(g).Error
}

func (r *gradingUIRepo) FindProvisionalGrade(ctx context.Context, submissionID, scorerID uint, final bool) (*models.ProvisionalGrade, error) {
	var pg models.ProvisionalGrade
	if err := r.db.WithContext(ctx).Where("submission_id = ? AND scorer_id = ? AND final = ?", submissionID, scorerID, final).First(&pg).Error; err != nil {
		return nil, err
	}
	return &pg, nil
}

func (r *gradingUIRepo) FindProvisionalGradeByID(ctx context.Context, id uint) (*models.ProvisionalGrade, error) {
	var pg models.ProvisionalGrade
	if err := r.db.WithContext(ctx).First(&pg, id).Error; err != nil {
		return nil, err
	}
	return &pg, nil
}

func (r *gradingUIRepo) SaveProvisionalGrade(ctx context.Context, pg *models.ProvisionalGrade) error {
	return r.db.WithContext(ctx).Save(pg).Error
}

func (r *gradingUIRepo) ListProvisionalGradesBySubmissions(ctx context.Context, submissionIDs []uint) ([]models.ProvisionalGrade, error) {
	if len(submissionIDs) == 0 {
		return nil, nil
	}
	var out []models.ProvisionalGrade
	err := r.db.WithContext(ctx).Where("submission_id IN ?", submissionIDs).Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingUIRepo) FindModerationSelection(ctx context.Context, assignmentID, studentID uint) (*models.ModerationSelection, error) {
	var s models.ModerationSelection
	if err := r.db.WithContext(ctx).Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gradingUIRepo) SaveModerationSelection(ctx context.Context, s *models.ModerationSelection) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *gradingUIRepo) ListModerationSelections(ctx context.Context, assignmentID uint) ([]models.ModerationSelection, error) {
	var out []models.ModerationSelection
	err := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID).Find(&out).Error
	return out, err
}

func (r *gradingUIRepo) SetAssignmentGradesPublished(ctx context.Context, assignmentID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.Assignment{}).Where("id = ?", assignmentID).
		Updates(map[string]interface{}{"grades_published_at": now, "updated_at": now}).Error
}

// --- custom grade statuses ---------------------------------------------

func (r *gradingUIRepo) ListCustomGradeStatuses(ctx context.Context, accountID uint, includeDeleted bool) ([]models.CustomGradeStatus, error) {
	var out []models.CustomGradeStatus
	q := r.db.WithContext(ctx).Where("account_id = ?", accountID)
	if !includeDeleted {
		q = q.Where("workflow_state = ?", "active")
	}
	err := q.Order("id ASC").Find(&out).Error
	return out, err
}

func (r *gradingUIRepo) FindCustomGradeStatus(ctx context.Context, id, accountID uint) (*models.CustomGradeStatus, error) {
	var s models.CustomGradeStatus
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gradingUIRepo) CreateCustomGradeStatus(ctx context.Context, s *models.CustomGradeStatus) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *gradingUIRepo) UpdateCustomGradeStatus(ctx context.Context, s *models.CustomGradeStatus) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *gradingUIRepo) ClearCustomGradeStatus(ctx context.Context, statusID uint) error {
	return r.db.WithContext(ctx).Model(&models.Submission{}).Where("custom_grade_status_id = ?", statusID).
		Update("custom_grade_status_id", nil).Error
}

// --- peer reviews ------------------------------------------------------

func (r *gradingUIRepo) DeletePeerReview(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.PeerReview{}, id).Error
}
