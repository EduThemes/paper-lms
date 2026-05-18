package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type AssignmentGroupService struct {
	groupRepo      repository.AssignmentGroupRepository
	assignmentRepo repository.AssignmentRepository
}

func NewAssignmentGroupService(groupRepo repository.AssignmentGroupRepository, assignmentRepo repository.AssignmentRepository) *AssignmentGroupService {
	return &AssignmentGroupService{
		groupRepo:      groupRepo,
		assignmentRepo: assignmentRepo,
	}
}

func (s *AssignmentGroupService) Create(ctx context.Context, group *models.AssignmentGroup) error {
	if group.Name == "" {
		return errors.New("assignment group name is required")
	}
	if group.WorkflowState == "" {
		group.WorkflowState = "available"
	}
	return s.groupRepo.Create(ctx, group)
}

// GetByID — Wave F. See CourseService.GetByID for the accountID contract.
// accountID==0 means "no tenant scope" (internal/background callers
// that have already validated tenant ownership upstream). Handler
// callers MUST pass callerAccountID(c).
func (s *AssignmentGroupService) GetByID(ctx context.Context, id, accountID uint) (*models.AssignmentGroup, error) {
	return s.groupRepo.FindByID(ctx, id, accountID)
}

func (s *AssignmentGroupService) Update(ctx context.Context, group *models.AssignmentGroup) error {
	return s.groupRepo.Update(ctx, group)
}

func (s *AssignmentGroupService) Delete(ctx context.Context, id uint) error {
	return s.groupRepo.Delete(ctx, id)
}

func (s *AssignmentGroupService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AssignmentGroup], error) {
	return s.groupRepo.ListByCourseID(ctx, courseID, params)
}
