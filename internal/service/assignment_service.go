package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type AssignmentService struct {
	repo repository.AssignmentRepository
}

func NewAssignmentService(repo repository.AssignmentRepository) *AssignmentService {
	return &AssignmentService{repo: repo}
}

func (s *AssignmentService) Create(ctx context.Context, assignment *models.Assignment) error {
	if assignment.Name == "" {
		return errors.New("assignment name is required")
	}
	if assignment.WorkflowState == "" {
		assignment.WorkflowState = "unpublished"
	}
	return s.repo.Create(ctx, assignment)
}

func (s *AssignmentService) GetByID(ctx context.Context, id uint) (*models.Assignment, error) {
	return s.repo.FindByID(ctx, id, 0)
}

func (s *AssignmentService) Update(ctx context.Context, assignment *models.Assignment) error {
	return s.repo.Update(ctx, assignment)
}

func (s *AssignmentService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *AssignmentService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Assignment], error) {
	return s.repo.ListByCourseID(ctx, courseID, params)
}
