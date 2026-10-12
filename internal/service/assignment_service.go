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

// GetByID — Wave F. See CourseService.GetByID for the accountID contract.
func (s *AssignmentService) GetByID(ctx context.Context, id, accountID uint) (*models.Assignment, error) {
	return s.repo.FindByID(ctx, id, accountID)
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

// ListVisibleByCourse lists a course's assignments for a caller. Staff see
// every non-deleted assignment; anyone else sees only published ones, and
// pagination is applied after that filter so counts don't leak drafts.
func (s *AssignmentService) ListVisibleByCourse(ctx context.Context, courseID uint, params repository.PaginationParams, includeUnpublished bool) (*repository.PaginatedResult[models.Assignment], error) {
	if includeUnpublished {
		return s.ListByCourse(ctx, courseID, params)
	}
	all, err := s.ListByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 100000})
	if err != nil {
		return nil, err
	}
	published := make([]models.Assignment, 0, len(all.Items))
	for i := range all.Items {
		if all.Items[i].IsPublished() {
			published = append(published, all.Items[i])
		}
	}
	page, perPage := params.Page, params.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = len(published)
	}
	start := (page - 1) * perPage
	if start > len(published) {
		start = len(published)
	}
	end := start + perPage
	if end > len(published) {
		end = len(published)
	}
	return &repository.PaginatedResult[models.Assignment]{
		Items:      published[start:end],
		TotalCount: int64(len(published)),
		Page:       page,
		PerPage:    perPage,
	}, nil
}
