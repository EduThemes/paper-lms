package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"gorm.io/gorm"
)

type EnrollmentTermService struct {
	enrollmentTermRepo *postgres.EnrollmentTermRepository
	db                 *gorm.DB
}

func NewEnrollmentTermService(enrollmentTermRepo *postgres.EnrollmentTermRepository, db *gorm.DB) *EnrollmentTermService {
	return &EnrollmentTermService{
		enrollmentTermRepo: enrollmentTermRepo,
		db:                 db,
	}
}

func (s *EnrollmentTermService) CreateTerm(ctx context.Context, term *models.EnrollmentTerm) error {
	if term.Name == "" {
		return errors.New("enrollment term name is required")
	}
	if term.StartAt != nil && term.EndAt != nil {
		if !term.StartAt.Before(*term.EndAt) {
			return errors.New("start_at must be before end_at")
		}
	}
	if term.WorkflowState == "" {
		term.WorkflowState = "active"
	}
	return s.enrollmentTermRepo.Create(ctx, term)
}

func (s *EnrollmentTermService) UpdateTerm(ctx context.Context, term *models.EnrollmentTerm) error {
	if term.StartAt != nil && term.EndAt != nil {
		if !term.StartAt.Before(*term.EndAt) {
			return errors.New("start_at must be before end_at")
		}
	}
	return s.enrollmentTermRepo.Update(ctx, term)
}

func (s *EnrollmentTermService) DeleteTerm(ctx context.Context, id uint) error {
	return s.enrollmentTermRepo.Delete(ctx, id)
}

func (s *EnrollmentTermService) GetTerm(ctx context.Context, id uint) (*models.EnrollmentTerm, error) {
	return s.enrollmentTermRepo.FindByID(ctx, id)
}

func (s *EnrollmentTermService) ListTerms(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.EnrollmentTerm], error) {
	return s.enrollmentTermRepo.ListByAccountID(ctx, accountID, params)
}

func (s *EnrollmentTermService) GetCurrentTerm(ctx context.Context, accountID uint) (*models.EnrollmentTerm, error) {
	return s.enrollmentTermRepo.FindCurrentTerm(ctx, accountID)
}

func (s *EnrollmentTermService) FindBySISTermID(ctx context.Context, sisTermID string) (*models.EnrollmentTerm, error) {
	return s.enrollmentTermRepo.FindBySISTermID(ctx, sisTermID)
}

func (s *EnrollmentTermService) GetTermCourses(ctx context.Context, termID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	var courses []models.Course
	var count int64

	query := s.db.WithContext(ctx).Model(&models.Course{}).Where("enrollment_term_id = ? AND workflow_state != ?", termID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&courses).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Course]{
		Items:      courses,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
