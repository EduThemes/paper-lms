package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type CoursePaceService struct {
	paceRepo       repository.CoursePaceRepository
	paceItemRepo   repository.CoursePaceModuleItemRepository
	moduleItemRepo repository.ModuleItemRepository
	assignmentRepo repository.AssignmentRepository
}

func NewCoursePaceService(paceRepo repository.CoursePaceRepository, paceItemRepo repository.CoursePaceModuleItemRepository, moduleItemRepo repository.ModuleItemRepository, assignmentRepo repository.AssignmentRepository) *CoursePaceService {
	return &CoursePaceService{paceRepo: paceRepo, paceItemRepo: paceItemRepo, moduleItemRepo: moduleItemRepo, assignmentRepo: assignmentRepo}
}

func (s *CoursePaceService) Create(ctx context.Context, pace *models.CoursePace) error {
	if pace.CourseID == 0 {
		return errors.New("course_id is required")
	}
	if pace.WorkflowState == "" {
		pace.WorkflowState = "unpublished"
	}
	return s.paceRepo.Create(ctx, pace)
}

func (s *CoursePaceService) GetByID(ctx context.Context, id uint) (*models.CoursePace, error) {
	return s.paceRepo.FindByID(ctx, id)
}

func (s *CoursePaceService) GetByCourseID(ctx context.Context, courseID uint) (*models.CoursePace, error) {
	return s.paceRepo.FindByCourseID(ctx, courseID)
}

func (s *CoursePaceService) GetByUserID(ctx context.Context, courseID uint, userID uint) (*models.CoursePace, error) {
	return s.paceRepo.FindByUserID(ctx, courseID, userID)
}

func (s *CoursePaceService) GetBySectionID(ctx context.Context, courseID uint, sectionID uint) (*models.CoursePace, error) {
	return s.paceRepo.FindBySectionID(ctx, courseID, sectionID)
}

func (s *CoursePaceService) Update(ctx context.Context, pace *models.CoursePace) error {
	return s.paceRepo.Update(ctx, pace)
}

func (s *CoursePaceService) Delete(ctx context.Context, id uint) error {
	return s.paceRepo.Delete(ctx, id)
}

func (s *CoursePaceService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CoursePace], error) {
	return s.paceRepo.ListByCourseID(ctx, courseID, params)
}

func (s *CoursePaceService) PublishPace(ctx context.Context, id uint) (*models.CoursePace, error) {
	pace, err := s.paceRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	pace.WorkflowState = "active"
	pace.PublishedAt = &now

	if err := s.paceRepo.Update(ctx, pace); err != nil {
		return nil, err
	}

	// Apply computed dates to linked assignments
	s.applyPaceDates(ctx, pace)

	return pace, nil
}

// applyPaceDates computes the timeline and updates assignment due dates for each
// module item that links to an assignment.
func (s *CoursePaceService) applyPaceDates(ctx context.Context, pace *models.CoursePace) {
	if s.moduleItemRepo == nil || s.assignmentRepo == nil {
		return
	}

	items, err := s.paceItemRepo.ListByPaceID(ctx, pace.ID)
	if err != nil || len(items) == 0 {
		return
	}

	startDate := time.Now()
	if pace.PublishedAt != nil {
		startDate = *pace.PublishedAt
	}

	currentDate := startDate
	for _, item := range items {
		currentDate = addBusinessDays(currentDate, item.Duration, pace.ExcludeWeekends)

		// Look up the module item (ContentTag) to find linked assignment
		moduleItem, err := s.moduleItemRepo.FindByID(ctx, item.ModuleItemID)
		if err != nil || moduleItem == nil {
			continue
		}
		if moduleItem.ContentType != "Assignment" || moduleItem.ContentID == nil {
			continue
		}

		assignment, err := s.assignmentRepo.FindByID(ctx, *moduleItem.ContentID, 0)
		if err != nil || assignment == nil {
			continue
		}

		projectedDate := currentDate
		assignment.DueAt = &projectedDate
		_ = s.assignmentRepo.Update(ctx, assignment)
	}
}

func (s *CoursePaceService) GetPaceItems(ctx context.Context, paceID uint) ([]models.CoursePaceModuleItem, error) {
	return s.paceItemRepo.ListByPaceID(ctx, paceID)
}

func (s *CoursePaceService) UpdatePaceItems(ctx context.Context, paceID uint, items []models.CoursePaceModuleItem) ([]models.CoursePaceModuleItem, error) {
	// Ensure all items are assigned to the correct pace
	for i := range items {
		items[i].CoursePaceID = paceID
	}

	if err := s.paceItemRepo.BulkUpsert(ctx, items); err != nil {
		return nil, err
	}

	return s.paceItemRepo.ListByPaceID(ctx, paceID)
}

// ComputeTimeline computes projected dates for each module item based on duration
// and whether weekends are excluded.
func (s *CoursePaceService) ComputeTimeline(ctx context.Context, paceID uint) ([]map[string]interface{}, error) {
	pace, err := s.paceRepo.FindByID(ctx, paceID)
	if err != nil {
		return nil, err
	}

	items, err := s.paceItemRepo.ListByPaceID(ctx, paceID)
	if err != nil {
		return nil, err
	}

	startDate := time.Now()
	if pace.PublishedAt != nil {
		startDate = *pace.PublishedAt
	}

	timeline := make([]map[string]interface{}, len(items))
	currentDate := startDate

	for i, item := range items {
		currentDate = addBusinessDays(currentDate, item.Duration, pace.ExcludeWeekends)

		timeline[i] = map[string]interface{}{
			"module_item_id": item.ModuleItemID,
			"duration":       item.Duration,
			"projected_date": currentDate.Format(time.RFC3339),
		}
	}

	return timeline, nil
}

// addBusinessDays adds the given number of days to a date, optionally skipping weekends.
func addBusinessDays(start time.Time, days int, excludeWeekends bool) time.Time {
	current := start
	remaining := days

	for remaining > 0 {
		current = current.AddDate(0, 0, 1)
		if excludeWeekends {
			weekday := current.Weekday()
			if weekday == time.Saturday || weekday == time.Sunday {
				continue
			}
		}
		remaining--
	}

	return current
}
