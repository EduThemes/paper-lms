package service

import (
	"context"
	"errors"
	"strings"

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

// ErrSISTermIDTaken is returned when another live term in the same
// account already uses the SIS term id.
var ErrSISTermIDTaken = errors.New("sis_term_id is already in use by another term")

// NormalizeSISTermID maps a blank SIS term id to NULL. The column sits
// under a partial UNIQUE index that skips NULLs; '' would be indexed and
// collide on the second term created without a SIS id (B071).
func NormalizeSISTermID(raw *string) *string {
	if raw == nil {
		return nil
	}
	v := strings.TrimSpace(*raw)
	if v == "" {
		return nil
	}
	return &v
}

// checkSISTermID refuses a SIS term id another term in the account owns,
// so the admin gets a clear message instead of a raw index violation.
func (s *EnrollmentTermService) checkSISTermID(ctx context.Context, term *models.EnrollmentTerm) error {
	term.SISTermID = NormalizeSISTermID(term.SISTermID)
	if term.SISTermID == nil || s.db == nil {
		return nil
	}
	var n int64
	q := s.db.WithContext(ctx).Model(&models.EnrollmentTerm{}).
		Where("account_id = ? AND sis_term_id = ?", term.AccountID, *term.SISTermID)
	if term.ID != 0 {
		q = q.Where("id <> ?", term.ID)
	}
	if err := q.Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ErrSISTermIDTaken
	}
	return nil
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
	if err := s.checkSISTermID(ctx, term); err != nil {
		return err
	}
	return s.enrollmentTermRepo.Create(ctx, term)
}

func (s *EnrollmentTermService) UpdateTerm(ctx context.Context, term *models.EnrollmentTerm) error {
	if term.StartAt != nil && term.EndAt != nil {
		if !term.StartAt.Before(*term.EndAt) {
			return errors.New("start_at must be before end_at")
		}
	}
	if err := s.checkSISTermID(ctx, term); err != nil {
		return err
	}
	return s.enrollmentTermRepo.Update(ctx, term)
}

// DeleteTerm soft-deletes a term within accountID.
func (s *EnrollmentTermService) DeleteTerm(ctx context.Context, id, accountID uint) error {
	return s.enrollmentTermRepo.Delete(ctx, id, accountID)
}

// GetTerm loads a term within accountID (cross-tenant ids miss).
func (s *EnrollmentTermService) GetTerm(ctx context.Context, id, accountID uint) (*models.EnrollmentTerm, error) {
	return s.enrollmentTermRepo.FindByID(ctx, id, accountID)
}

func (s *EnrollmentTermService) ListTerms(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.EnrollmentTerm], error) {
	return s.enrollmentTermRepo.ListByAccountID(ctx, accountID, params)
}

func (s *EnrollmentTermService) GetCurrentTerm(ctx context.Context, accountID uint) (*models.EnrollmentTerm, error) {
	return s.enrollmentTermRepo.FindCurrentTerm(ctx, accountID)
}

func (s *EnrollmentTermService) FindBySISTermID(ctx context.Context, sisTermID string, accountID uint) (*models.EnrollmentTerm, error) {
	return s.enrollmentTermRepo.FindBySISTermID(ctx, sisTermID, accountID)
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
