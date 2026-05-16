package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type submissionRepo struct {
	db *gorm.DB
}

func NewSubmissionRepository(db *gorm.DB) repository.SubmissionRepository {
	return &submissionRepo{db: db}
}

func (r *submissionRepo) Create(ctx context.Context, submission *models.Submission) error {
	return r.db.WithContext(ctx).Create(submission).Error
}

func (r *submissionRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Submission, error) {
	var submission models.Submission
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Scope through assignment->course->account_id (deep 2-level subquery).
		q = q.Where("assignment_id IN (SELECT id FROM assignments WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?))", accountID)
	}
	if err := q.First(&submission, id).Error; err != nil {
		return nil, err
	}
	return &submission, nil
}

func (r *submissionRepo) FindByAssignmentAndUser(ctx context.Context, assignmentID, userID uint) (*models.Submission, error) {
	var submission models.Submission
	if err := r.db.WithContext(ctx).Where("assignment_id = ? AND user_id = ?", assignmentID, userID).First(&submission).Error; err != nil {
		return nil, err
	}
	return &submission, nil
}

func (r *submissionRepo) FindByIDs(ctx context.Context, ids []uint) ([]models.Submission, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var submissions []models.Submission
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&submissions).Error; err != nil {
		return nil, err
	}
	return submissions, nil
}

func (r *submissionRepo) FindByAssignmentAndUserIDs(ctx context.Context, assignmentID uint, userIDs []uint) ([]models.Submission, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var submissions []models.Submission
	if err := r.db.WithContext(ctx).Where("assignment_id = ? AND user_id IN ?", assignmentID, userIDs).Find(&submissions).Error; err != nil {
		return nil, err
	}
	return submissions, nil
}

func (r *submissionRepo) RunInTransaction(ctx context.Context, fn func(txRepo repository.SubmissionRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&submissionRepo{db: tx})
	})
}

func (r *submissionRepo) Update(ctx context.Context, submission *models.Submission) error {
	return r.db.WithContext(ctx).Save(submission).Error
}

func (r *submissionRepo) ListByAssignmentID(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	var submissions []models.Submission
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Submission{}).Where("assignment_id = ?", assignmentID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&submissions).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Submission]{
		Items:      submissions,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *submissionRepo) ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.Submission, error) {
	var submissions []models.Submission

	subQuery := r.db.Model(&models.Assignment{}).Select("id").Where("course_id = ? AND workflow_state != ?", courseID, "deleted")

	if err := r.db.WithContext(ctx).Where("user_id = ? AND assignment_id IN (?)", userID, subQuery).Find(&submissions).Error; err != nil {
		return nil, err
	}
	return submissions, nil
}

func (r *submissionRepo) ListByUserAndAssignmentIDs(ctx context.Context, userID uint, assignmentIDs []uint) ([]models.Submission, error) {
	if len(assignmentIDs) == 0 {
		return nil, nil
	}
	var submissions []models.Submission
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND assignment_id IN ?", userID, assignmentIDs).
		Find(&submissions).Error; err != nil {
		return nil, err
	}
	return submissions, nil
}

func (r *submissionRepo) BulkListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	var submissions []models.Submission
	var count int64

	subQuery := r.db.Model(&models.Assignment{}).Select("id").Where("course_id = ? AND workflow_state != ?", courseID, "deleted")

	query := r.db.WithContext(ctx).Model(&models.Submission{}).Where("assignment_id IN (?)", subQuery)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&submissions).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Submission]{
		Items:      submissions,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *submissionRepo) PostGradesByAssignment(ctx context.Context, assignmentID uint, postedAt *time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.Submission{}).
		Where("assignment_id = ? AND score IS NOT NULL", assignmentID).
		Update("posted_at", postedAt).Error
}
