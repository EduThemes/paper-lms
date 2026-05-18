package service

// Blueprint associations / subscriptions — the methods that wire a
// blueprint template to one or more child courses. Each association is
// stored as a BlueprintSubscription row.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint).

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ListAssociatedCourses lists subscriptions (associated courses) for a template.
func (s *BlueprintService) ListAssociatedCourses(ctx context.Context, templateID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintSubscription], error) {
	return s.subRepo.ListByTemplateID(ctx, templateID, params)
}

// UpdateAssociations reconciles the set of associated courses for a template.
// courseIDs that are not yet associated will be added; existing associations not in courseIDs will be removed.
func (s *BlueprintService) UpdateAssociations(ctx context.Context, templateID uint, courseIDs []uint) error {
	if templateID == 0 {
		return errors.New("template_id is required")
	}

	// Fetch all current subscriptions for this template
	existing, err := s.subRepo.ListByTemplateID(ctx, templateID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		return err
	}

	// Build a set of desired course IDs
	desired := make(map[uint]bool, len(courseIDs))
	for _, id := range courseIDs {
		desired[id] = true
	}

	// Build a set of currently active course IDs
	current := make(map[uint]uint, len(existing.Items)) // childCourseID -> subscriptionID
	for _, sub := range existing.Items {
		current[sub.ChildCourseID] = sub.ID
	}

	// Remove subscriptions that are no longer desired
	for childID, subID := range current {
		if !desired[childID] {
			if err := s.subRepo.Delete(ctx, subID); err != nil {
				return err
			}
		}
	}

	// Add new subscriptions
	for _, courseID := range courseIDs {
		if _, exists := current[courseID]; !exists {
			sub := &models.BlueprintSubscription{
				BlueprintTemplateID: templateID,
				ChildCourseID:       courseID,
				WorkflowState:       "active",
			}
			if err := s.subRepo.Create(ctx, sub); err != nil {
				return err
			}
		}
	}

	return nil
}

// ListSubscriptions lists subscriptions for a child course.
func (s *BlueprintService) ListSubscriptions(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintSubscription], error) {
	return s.subRepo.ListByChildCourseID(ctx, courseID, params)
}

// GetSubscription returns a subscription by ID.
func (s *BlueprintService) GetSubscription(ctx context.Context, id uint) (*models.BlueprintSubscription, error) {
	return s.subRepo.FindByID(ctx, id)
}

// ListSubscriptionMigrations lists migrations associated with a subscription.
func (s *BlueprintService) ListSubscriptionMigrations(ctx context.Context, subscriptionID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintMigration], error) {
	return s.migRepo.ListBySubscriptionID(ctx, subscriptionID, params)
}
