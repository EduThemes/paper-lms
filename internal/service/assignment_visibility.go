package service

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SetStudentVisibility wires the per-student assignment visibility
// (Mastery Paths) used by ListVisibleByCourseFor.
func (s *AssignmentService) SetStudentVisibility(v AssignmentVisibility) {
	s.visibility = v
}

// ListVisibleByCourseFor is ListVisibleByCourse for a specific caller:
// a non-staff caller additionally loses the Mastery Paths assignments
// not released to them (G080). Pagination is applied after filtering so
// counts do not reveal hidden assignments.
func (s *AssignmentService) ListVisibleByCourseFor(ctx context.Context, courseID uint, params repository.PaginationParams, includeUnpublished bool, userID uint) (*repository.PaginatedResult[models.Assignment], error) {
	if includeUnpublished || s.visibility == nil || userID == 0 {
		return s.ListVisibleByCourse(ctx, courseID, params, includeUnpublished)
	}
	hidden, err := s.visibility.HiddenAssignmentIDs(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}
	if len(hidden) == 0 {
		return s.ListVisibleByCourse(ctx, courseID, params, false)
	}
	all, err := s.ListVisibleByCourse(ctx, courseID, repository.PaginationParams{Page: 1, PerPage: 100000}, false)
	if err != nil {
		return nil, err
	}
	visible := make([]models.Assignment, 0, len(all.Items))
	for _, a := range all.Items {
		if !hidden[a.ID] {
			visible = append(visible, a)
		}
	}
	page, perPage := params.Page, params.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = len(visible)
	}
	start := (page - 1) * perPage
	if start > len(visible) {
		start = len(visible)
	}
	end := start + perPage
	if end > len(visible) {
		end = len(visible)
	}
	return &repository.PaginatedResult[models.Assignment]{
		Items:      visible[start:end],
		TotalCount: int64(len(visible)),
		Page:       page,
		PerPage:    perPage,
	}, nil
}
