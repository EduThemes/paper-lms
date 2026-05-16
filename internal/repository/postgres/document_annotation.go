package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type documentAnnotationRepo struct {
	db *gorm.DB
}

func NewDocumentAnnotationRepository(db *gorm.DB) repository.DocumentAnnotationRepository {
	return &documentAnnotationRepo{db: db}
}

func (r *documentAnnotationRepo) Create(ctx context.Context, annotation *models.DocumentAnnotation) error {
	return r.db.WithContext(ctx).Create(annotation).Error
}

func (r *documentAnnotationRepo) FindByID(ctx context.Context, id, accountID uint) (*models.DocumentAnnotation, error) {
	var annotation models.DocumentAnnotation
	q := r.db.WithContext(ctx).Preload("User").Preload("Replies", "workflow_state = ?", "active").Preload("Replies.User")
	if accountID != 0 {
		// Scope through submission->assignment->course (deep 3-level subquery).
		q = q.Where("submission_id IN (SELECT id FROM submissions WHERE assignment_id IN (SELECT id FROM assignments WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)))", accountID)
	}
	if err := q.First(&annotation, id).Error; err != nil {
		return nil, err
	}
	return &annotation, nil
}

func (r *documentAnnotationRepo) Update(ctx context.Context, annotation *models.DocumentAnnotation) error {
	return r.db.WithContext(ctx).Save(annotation).Error
}

func (r *documentAnnotationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.DocumentAnnotation{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *documentAnnotationRepo) ListBySubmissionID(ctx context.Context, submissionID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DocumentAnnotation], error) {
	var annotations []models.DocumentAnnotation
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DocumentAnnotation{}).
		Where("submission_id = ? AND workflow_state != ? AND parent_annotation_id IS NULL", submissionID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Preload("User").Preload("Replies", "workflow_state = ?", "active").Preload("Replies.User").
		Offset(offset).Limit(params.PerPage).Order("page_number ASC, selection_start ASC, created_at ASC").
		Find(&annotations).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DocumentAnnotation]{
		Items:      annotations,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *documentAnnotationRepo) ListBySubmissionAndPage(ctx context.Context, submissionID uint, pageNumber int) ([]models.DocumentAnnotation, error) {
	var annotations []models.DocumentAnnotation
	if err := r.db.WithContext(ctx).Preload("User").Preload("Replies", "workflow_state = ?", "active").Preload("Replies.User").
		Where("submission_id = ? AND page_number = ? AND workflow_state != ? AND parent_annotation_id IS NULL", submissionID, pageNumber, "deleted").
		Order("selection_start ASC, created_at ASC").
		Find(&annotations).Error; err != nil {
		return nil, err
	}
	return annotations, nil
}

func (r *documentAnnotationRepo) CountBySubmissionID(ctx context.Context, submissionID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.DocumentAnnotation{}).
		Where("submission_id = ? AND workflow_state != ?", submissionID, "deleted").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *documentAnnotationRepo) ListReplies(ctx context.Context, parentAnnotationID uint) ([]models.DocumentAnnotation, error) {
	var replies []models.DocumentAnnotation
	if err := r.db.WithContext(ctx).Preload("User").
		Where("parent_annotation_id = ? AND workflow_state != ?", parentAnnotationID, "deleted").
		Order("created_at ASC").
		Find(&replies).Error; err != nil {
		return nil, err
	}
	return replies, nil
}

func (r *documentAnnotationRepo) Resolve(ctx context.Context, annotationID uint, resolvedByUserID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.DocumentAnnotation{}).Where("id = ?", annotationID).
		Updates(map[string]interface{}{
			"resolved_at":         &now,
			"resolved_by_user_id": &resolvedByUserID,
			"workflow_state":      "resolved",
		}).Error
}

func (r *documentAnnotationRepo) Unresolve(ctx context.Context, annotationID uint) error {
	return r.db.WithContext(ctx).Model(&models.DocumentAnnotation{}).Where("id = ?", annotationID).
		Updates(map[string]interface{}{
			"resolved_at":         nil,
			"resolved_by_user_id": nil,
			"workflow_state":      "active",
		}).Error
}
