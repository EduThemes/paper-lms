package service

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type OverrideService struct {
	overrideRepo        repository.AssignmentOverrideRepository
	overrideStudentRepo repository.AssignmentOverrideStudentRepository
	enrollmentRepo      repository.EnrollmentRepository
	sectionRepo         repository.SectionRepository
}

func NewOverrideService(
	overrideRepo repository.AssignmentOverrideRepository,
	overrideStudentRepo repository.AssignmentOverrideStudentRepository,
	enrollmentRepo repository.EnrollmentRepository,
	sectionRepo repository.SectionRepository,
) *OverrideService {
	return &OverrideService{
		overrideRepo:        overrideRepo,
		overrideStudentRepo: overrideStudentRepo,
		enrollmentRepo:      enrollmentRepo,
		sectionRepo:         sectionRepo,
	}
}

func (s *OverrideService) CreateOverride(ctx context.Context, override *models.AssignmentOverride) error {
	if override.WorkflowState == "" {
		override.WorkflowState = "active"
	}
	return s.overrideRepo.Create(ctx, override)
}

func (s *OverrideService) GetOverride(ctx context.Context, id uint) (*models.AssignmentOverride, error) {
	return s.overrideRepo.FindByID(ctx, id)
}

func (s *OverrideService) UpdateOverride(ctx context.Context, override *models.AssignmentOverride) error {
	return s.overrideRepo.Update(ctx, override)
}

func (s *OverrideService) DeleteOverride(ctx context.Context, id uint) error {
	return s.overrideRepo.Delete(ctx, id)
}

func (s *OverrideService) ListOverrides(ctx context.Context, assignmentID uint) ([]models.AssignmentOverride, error) {
	return s.overrideRepo.ListByAssignmentID(ctx, assignmentID)
}

func (s *OverrideService) AddStudent(ctx context.Context, overrideID, userID, assignmentID uint) error {
	student := &models.AssignmentOverrideStudent{
		AssignmentOverrideID: overrideID,
		UserID:               userID,
		AssignmentID:         assignmentID,
	}
	return s.overrideStudentRepo.Create(ctx, student)
}

func (s *OverrideService) RemoveStudent(ctx context.Context, overrideID, userID uint) error {
	return s.overrideStudentRepo.Delete(ctx, overrideID, userID)
}

func (s *OverrideService) ListStudents(ctx context.Context, overrideID uint) ([]models.AssignmentOverrideStudent, error) {
	return s.overrideStudentRepo.ListByOverrideID(ctx, overrideID)
}

// GetEffectiveDates determines the effective due_at, unlock_at, and lock_at for a
// specific user on a specific assignment. It checks student overrides first, then
// section overrides, then returns nil values (caller should fall back to assignment defaults).
func (s *OverrideService) GetEffectiveDates(ctx context.Context, assignmentID, userID uint) (dueAt, unlockAt, lockAt *time.Time) {
	// 1. Check for student-specific overrides
	studentOverrides, err := s.overrideStudentRepo.ListByUserAndAssignment(ctx, userID, assignmentID)
	if err == nil && len(studentOverrides) > 0 {
		override, err := s.overrideRepo.FindByID(ctx, studentOverrides[0].AssignmentOverrideID)
		if err == nil {
			return override.DueAt, override.UnlockAt, override.LockAt
		}
	}

	// 2. Check for section-based overrides
	// Find the user's enrollment to determine their section
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, userID)
	if err == nil {
		for _, enrollment := range enrollments {
			if enrollment.CourseSectionID == nil {
				continue
			}
			// Look for an override matching this section
			overrides, err := s.overrideRepo.ListByAssignmentID(ctx, assignmentID)
			if err != nil {
				continue
			}
			for _, override := range overrides {
				if override.CourseSectionID != nil && *override.CourseSectionID == *enrollment.CourseSectionID {
					return override.DueAt, override.UnlockAt, override.LockAt
				}
			}
		}
	}

	// 3. No overrides found — caller uses assignment defaults
	return nil, nil, nil
}
