package service

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

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

// DeleteOverride soft-deletes the override and removes its student rows.
// Leaving them behind kept the (assignment_id, user_id) unique slot taken,
// so the student could never get another override on that assignment
// (B112).
func (s *OverrideService) DeleteOverride(ctx context.Context, id uint) error {
	if err := s.overrideRepo.Delete(ctx, id); err != nil {
		return err
	}
	if s.overrideStudentRepo != nil {
		if students, err := s.overrideStudentRepo.ListByOverrideID(ctx, id); err == nil {
			for _, st := range students {
				_ = s.overrideStudentRepo.Delete(ctx, id, st.UserID)
			}
		}
	}
	return nil
}

func (s *OverrideService) ListOverrides(ctx context.Context, assignmentID uint) ([]models.AssignmentOverride, error) {
	return s.overrideRepo.ListByAssignmentID(ctx, assignmentID)
}

// IsCourseStudent reports whether userID holds an active StudentEnrollment
// in courseID (tenant-scoped by accountID).
func (s *OverrideService) IsCourseStudent(ctx context.Context, courseID, userID, accountID uint) bool {
	if s.enrollmentRepo == nil {
		return false
	}
	e, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	return err == nil && e != nil && e.Type == "StudentEnrollment"
}

func (s *OverrideService) AddStudent(ctx context.Context, overrideID, userID, assignmentID uint) error {
	// Drop stale rows left by overrides deleted before DeleteOverride
	// cleaned up after itself; they would otherwise block this insert.
	if rows, err := s.overrideStudentRepo.ListByUserAndAssignment(ctx, userID, assignmentID); err == nil {
		for _, r := range rows {
			if r.AssignmentOverrideID == overrideID {
				continue
			}
			if _, err := s.overrideRepo.FindByID(ctx, r.AssignmentOverrideID); errors.Is(err, gorm.ErrRecordNotFound) {
				_ = s.overrideStudentRepo.Delete(ctx, r.AssignmentOverrideID, userID)
			}
		}
	}
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
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, userID, 0)
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

// EffectiveDatesFor resolves the due/unlock/lock dates that actually
// apply to userID on assignment, starting from the assignment's own
// dates and replacing them with the most lenient applicable override
// (an ad-hoc student override or one for a section the student is
// enrolled in, in the assignment's course). Mirrors Canvas: when several
// overrides apply, the one with the latest due date wins, and "no due
// date" counts as the latest of all.
func (s *OverrideService) EffectiveDatesFor(ctx context.Context, assignment *models.Assignment, userID uint) (dueAt, unlockAt, lockAt *time.Time) {
	dueAt, unlockAt, lockAt, _ = s.EffectiveDatesWithOverride(ctx, assignment, userID)
	return
}

// EffectiveDatesWithOverride is EffectiveDatesFor that also reports
// whether any override applies to the student (an assignment that is
// only visible to overrides is not assigned to anyone without one).
func (s *OverrideService) EffectiveDatesWithOverride(ctx context.Context, assignment *models.Assignment, userID uint) (dueAt, unlockAt, lockAt *time.Time, hasOverride bool) {
	if assignment == nil {
		return nil, nil, nil, false
	}
	dueAt, unlockAt, lockAt = assignment.DueAt, assignment.UnlockAt, assignment.LockAt
	if s == nil {
		return
	}

	var applicable []models.AssignmentOverride
	seen := map[uint]bool{}
	add := func(o models.AssignmentOverride) {
		if o.AssignmentID != assignment.ID || o.WorkflowState == "deleted" || seen[o.ID] {
			return
		}
		seen[o.ID] = true
		applicable = append(applicable, o)
	}

	if s.overrideStudentRepo != nil {
		if rows, err := s.overrideStudentRepo.ListByUserAndAssignment(ctx, userID, assignment.ID); err == nil {
			for _, r := range rows {
				if o, err := s.overrideRepo.FindByID(ctx, r.AssignmentOverrideID); err == nil && o != nil {
					add(*o)
				}
			}
		}
	}

	if s.enrollmentRepo != nil && s.overrideRepo != nil {
		var sectionIDs []uint
		if enrollments, err := s.enrollmentRepo.ListByUserID(ctx, userID, 0); err == nil {
			for _, e := range enrollments {
				if e.CourseID == assignment.CourseID && e.CourseSectionID != nil {
					sectionIDs = append(sectionIDs, *e.CourseSectionID)
				}
			}
		}
		if len(sectionIDs) > 0 {
			if overrides, err := s.overrideRepo.ListByAssignmentID(ctx, assignment.ID); err == nil {
				for _, o := range overrides {
					if o.CourseSectionID == nil {
						continue
					}
					for _, sid := range sectionIDs {
						if *o.CourseSectionID == sid {
							add(o)
							break
						}
					}
				}
			}
		}
	}

	if len(applicable) == 0 {
		return
	}
	best := applicable[0]
	for _, o := range applicable[1:] {
		if laterDue(o.DueAt, best.DueAt) {
			best = o
		}
	}
	return best.DueAt, best.UnlockAt, best.LockAt, true
}

// laterDue reports whether a is a strictly more lenient due date than b
// (nil = no due date = most lenient).
func laterDue(a, b *time.Time) bool {
	if b == nil {
		return false
	}
	if a == nil {
		return true
	}
	return a.After(*b)
}
