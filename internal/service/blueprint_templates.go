package service

// Blueprint template CRUD — the methods that operate on the blueprint
// template itself (one per source course). Associations / subscriptions
// live in blueprint_associations.go; sync orchestration lives in
// blueprint_sync.go.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint).

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// GetOrCreateTemplate returns the existing template for a course, or creates a new one.
func (s *BlueprintService) GetOrCreateTemplate(ctx context.Context, courseID uint) (*models.BlueprintTemplate, error) {
	if courseID == 0 {
		return nil, errors.New("course_id is required")
	}

	template, err := s.tmplRepo.FindByCourseID(ctx, courseID)
	if err == nil {
		return template, nil
	}

	// Create a new template for this course
	template = &models.BlueprintTemplate{
		CourseID:               courseID,
		DefaultRestrictions:    "{}",
		UseDefaultRestrictions: true,
		WorkflowState:          "active",
	}
	if err := s.tmplRepo.Create(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

// GetTemplate returns a template by ID.
func (s *BlueprintService) GetTemplate(ctx context.Context, id uint) (*models.BlueprintTemplate, error) {
	return s.tmplRepo.FindByID(ctx, id)
}

// UpdateTemplate updates an existing template.
func (s *BlueprintService) UpdateTemplate(ctx context.Context, template *models.BlueprintTemplate) error {
	if template.ID == 0 {
		return errors.New("template id is required")
	}
	return s.tmplRepo.Update(ctx, template)
}

// ListTemplates returns templates for a course.
func (s *BlueprintService) ListTemplates(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintTemplate], error) {
	return s.tmplRepo.ListByCourseID(ctx, courseID, params)
}
