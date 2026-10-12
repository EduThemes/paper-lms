package service

// Blueprint associations / subscriptions — the methods that wire a
// blueprint template to one or more child courses. Each association is
// stored as a BlueprintSubscription row.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint).

import (
	"context"
	"errors"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ErrBlueprintInvalidAssociation is returned when a course id passed to
// ModifyAssociations may not be associated with the blueprint. The
// message deliberately does not say why (missing, other tenant, no
// rights) so the endpoint can't be used to probe course ids.
var ErrBlueprintInvalidAssociation = errors.New("invalid course_ids_to_add")

// BlueprintCaller identifies who is changing a blueprint's associations.
type BlueprintCaller struct {
	UserID    uint
	AccountID uint
	IsAdmin   bool
}

// ListAssociatedCourses lists subscriptions (associated courses) for a template.
func (s *BlueprintService) ListAssociatedCourses(ctx context.Context, templateID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintSubscription], error) {
	return s.subRepo.ListByTemplateID(ctx, templateID, params)
}

// ModifyAssociations adds and removes associated (child) courses of a
// blueprint template.
//
// It works from the add/remove deltas against the FULL current set
// (B040: the handler used to rebuild the desired set from one paginated
// page of 10 and every association beyond it was deleted).
//
// Every course to add is authorized first (B037, Canvas
// MasterTemplatesController#update_associations): it must be a live
// course in the blueprint's own account, not the blueprint itself, not
// a blueprint with children of its own, not already associated with a
// different blueprint, and the caller must be an admin or an active
// teacher of it. Any failure rejects the whole request with
// ErrBlueprintInvalidAssociation before anything is written.
func (s *BlueprintService) ModifyAssociations(ctx context.Context, template *models.BlueprintTemplate, caller BlueprintCaller, add, remove []uint) error {
	if template == nil || template.ID == 0 {
		return errors.New("template_id is required")
	}

	removeSet := make(map[uint]bool, len(remove))
	for _, id := range remove {
		removeSet[id] = true
	}
	toAdd := make([]uint, 0, len(add))
	seen := make(map[uint]bool, len(add))
	for _, id := range add {
		if id == 0 || seen[id] || removeSet[id] {
			continue
		}
		seen[id] = true
		toAdd = append(toAdd, id)
	}

	if len(toAdd) > 0 {
		if err := s.authorizeChildCourses(ctx, template, caller, toAdd); err != nil {
			return err
		}
	}

	existing, err := s.subRepo.ListByTemplateID(ctx, template.ID, bigPage)
	if err != nil {
		return err
	}
	current := make(map[uint]uint, len(existing.Items)) // childCourseID -> subscriptionID
	for _, sub := range existing.Items {
		current[sub.ChildCourseID] = sub.ID
	}

	for childID := range removeSet {
		if subID, ok := current[childID]; ok {
			if err := s.subRepo.Delete(ctx, subID); err != nil {
				return err
			}
		}
	}

	for _, courseID := range toAdd {
		if _, exists := current[courseID]; exists {
			continue
		}
		// subRepo.Create revives a previously removed (soft-deleted)
		// subscription instead of colliding on idx_template_child (B135).
		sub := &models.BlueprintSubscription{
			BlueprintTemplateID: template.ID,
			ChildCourseID:       courseID,
			WorkflowState:       "active",
		}
		if err := s.subRepo.Create(ctx, sub); err != nil {
			return err
		}
	}

	return nil
}

// authorizeChildCourses applies the association rules to every course
// id in ids. Fails closed when the course/enrollment repositories are
// not wired.
func (s *BlueprintService) authorizeChildCourses(ctx context.Context, template *models.BlueprintTemplate, caller BlueprintCaller, ids []uint) error {
	if s.courseRepo == nil || s.enrollmentRepo == nil || caller.AccountID == 0 {
		return ErrBlueprintInvalidAssociation
	}
	blueprint, err := s.courseRepo.FindByID(ctx, template.CourseID, caller.AccountID)
	if err != nil || blueprint == nil {
		return ErrBlueprintInvalidAssociation
	}
	for _, id := range ids {
		if id == template.CourseID {
			return fmt.Errorf("%w: a blueprint cannot be associated with itself", ErrBlueprintInvalidAssociation)
		}
		child, err := s.courseRepo.FindByID(ctx, id, blueprint.AccountID)
		if err != nil || child == nil || child.AccountID != blueprint.AccountID || child.WorkflowState == models.CourseDeleted {
			return ErrBlueprintInvalidAssociation
		}
		if !caller.IsAdmin {
			e, err := s.enrollmentRepo.FindByUserAndCourse(ctx, caller.UserID, id, caller.AccountID)
			if err != nil || e == nil || e.WorkflowState != "active" || e.Type != "TeacherEnrollment" {
				return ErrBlueprintInvalidAssociation
			}
		}
		// Already a child of a different blueprint?
		if subs, err := s.subRepo.ListByChildCourseID(ctx, id, bigPage); err == nil {
			for _, sub := range subs.Items {
				if sub.BlueprintTemplateID != template.ID && sub.WorkflowState == "active" {
					return fmt.Errorf("%w: course is already associated with another blueprint", ErrBlueprintInvalidAssociation)
				}
			}
		}
		// Itself a blueprint with children?
		if tmpl, err := s.tmplRepo.FindByCourseID(ctx, id); err == nil && tmpl != nil {
			if subs, err := s.subRepo.ListByTemplateID(ctx, tmpl.ID, repository.PaginationParams{Page: 1, PerPage: 1}); err == nil && subs.TotalCount > 0 {
				return fmt.Errorf("%w: a blueprint course cannot be associated", ErrBlueprintInvalidAssociation)
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
