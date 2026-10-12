package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// SectionService manages course sections (G109: rename, dates, delete)
// and cross-listing (G108).
//
// Cross-listing (Canvas CourseSection#crosslist_to_course) moves a
// section, with its enrollments, into another course of the same
// account; the original course is remembered in nonxlist_course_id so
// de-cross-listing can move it back. Submissions and grades stay with the
// assignments of the course they were made in — moving a section does not
// carry them across (Canvas warns about the same).
type SectionService struct {
	sections    repository.SectionAdminRepository
	enrollments repository.EnrollmentAdminRepository
	courses     repository.CourseRepository
}

func NewSectionService(sections repository.SectionAdminRepository, enrollments repository.EnrollmentAdminRepository, courses repository.CourseRepository) *SectionService {
	return &SectionService{sections: sections, enrollments: enrollments, courses: courses}
}

var (
	ErrSectionNotFound        = errors.New("section not found")
	ErrSectionNameRequired    = errors.New("section name is required")
	ErrSectionDates           = errors.New("end_at must be after start_at")
	ErrSectionHasEnrollments  = errors.New("a section with enrollments can't be deleted; move or remove its people first")
	ErrSectionNotCrosslisted  = errors.New("section is not cross-listed")
	ErrCrosslistSameCourse    = errors.New("the section already belongs to that course")
	ErrCrosslistCourseInvalid = errors.New("target course not found")
)

// Find loads a live section whose course is in accountID (0 = any).
func (s *SectionService) Find(ctx context.Context, id, accountID uint) (*models.CourseSection, error) {
	sec, err := s.sections.FindInAccount(ctx, id, accountID)
	if err != nil || sec == nil {
		return nil, ErrSectionNotFound
	}
	return sec, nil
}

// SectionUpdate is a partial section edit.
type SectionUpdate struct {
	Name                              *string
	StartAtSet                        bool
	StartAt                           *time.Time
	EndAtSet                          bool
	EndAt                             *time.Time
	RestrictEnrollmentsToSectionDates *bool
}

// Update renames a section and edits its dates.
func (s *SectionService) Update(ctx context.Context, sec *models.CourseSection, upd SectionUpdate) error {
	if upd.Name != nil {
		name := strings.TrimSpace(SanitizePlainText(*upd.Name))
		if name == "" {
			return ErrSectionNameRequired
		}
		sec.Name = name
	}
	if upd.StartAtSet {
		sec.StartAt = upd.StartAt
	}
	if upd.EndAtSet {
		sec.EndAt = upd.EndAt
	}
	if sec.StartAt != nil && sec.EndAt != nil && sec.EndAt.Before(*sec.StartAt) {
		return ErrSectionDates
	}
	if upd.RestrictEnrollmentsToSectionDates != nil {
		sec.RestrictEnrollmentsToSectionDates = *upd.RestrictEnrollmentsToSectionDates
	}
	return s.sections.Save(ctx, sec)
}

// Delete soft-deletes an empty section (Canvas refuses to delete a
// section that still has enrollments).
func (s *SectionService) Delete(ctx context.Context, sec *models.CourseSection) error {
	n, err := s.enrollments.CountLiveInSection(ctx, sec.ID)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrSectionHasEnrollments
	}
	sec.WorkflowState = "deleted"
	return s.sections.Save(ctx, sec)
}

// CrosslistTo resolves the target course inside accountID (0 = any) and
// cross-lists the section into it. The target must be in the same account
// as the section's current course.
func (s *SectionService) CrosslistTo(ctx context.Context, sec *models.CourseSection, targetCourseID, accountID uint) error {
	target, err := s.courses.FindByID(ctx, targetCourseID, accountID)
	if err != nil || target == nil {
		return ErrCrosslistCourseInvalid
	}
	source, err := s.courses.FindByID(ctx, sec.CourseID, 0)
	if err != nil || source == nil || source.AccountID != target.AccountID {
		return ErrCrosslistCourseInvalid
	}
	return s.Crosslist(ctx, sec, target)
}

// Crosslist moves the section and its enrollments into target. The
// caller has already checked the user may manage both courses and that
// target is in the section's account. Cross-listing back to the original
// course de-cross-lists it.
func (s *SectionService) Crosslist(ctx context.Context, sec *models.CourseSection, target *models.Course) error {
	if target == nil || target.WorkflowState == models.CourseDeleted {
		return ErrCrosslistCourseInvalid
	}
	if target.ID == sec.CourseID {
		return ErrCrosslistSameCourse
	}
	nonxlist := sec.NonxlistCourseID
	if nonxlist == nil {
		orig := sec.CourseID
		nonxlist = &orig
	}
	if *nonxlist == target.ID {
		nonxlist = nil // moving home: no longer cross-listed
	}
	return s.sections.MoveToCourse(ctx, sec, target.ID, nonxlist)
}

// Uncrosslist moves a cross-listed section back to its original course.
func (s *SectionService) Uncrosslist(ctx context.Context, sec *models.CourseSection) error {
	if sec.NonxlistCourseID == nil {
		return ErrSectionNotCrosslisted
	}
	return s.sections.MoveToCourse(ctx, sec, *sec.NonxlistCourseID, nil)
}

// CrosslistBySIS applies one xlists.csv row inside accountID: status
// "active" cross-lists the section into the course, "deleted"
// de-cross-lists it (only when it is cross-listed into that course).
func (s *SectionService) CrosslistBySIS(ctx context.Context, sec *models.CourseSection, target *models.Course, active bool) error {
	if active {
		if target.ID == sec.CourseID {
			return nil // already there
		}
		return s.Crosslist(ctx, sec, target)
	}
	if sec.NonxlistCourseID == nil || sec.CourseID != target.ID {
		return nil
	}
	return s.Uncrosslist(ctx, sec)
}
