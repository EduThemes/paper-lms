package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type CourseService struct {
	courseRepo     repository.CourseRepository
	enrollmentRepo repository.EnrollmentRepository
	sectionRepo    repository.SectionRepository
}

func NewCourseService(courseRepo repository.CourseRepository, enrollmentRepo repository.EnrollmentRepository, sectionRepo repository.SectionRepository) *CourseService {
	return &CourseService{
		courseRepo:     courseRepo,
		enrollmentRepo: enrollmentRepo,
		sectionRepo:    sectionRepo,
	}
}

func (s *CourseService) Create(ctx context.Context, course *models.Course, creatorID uint) error {
	if course.Name == "" {
		return errors.New("course name is required")
	}
	if course.CourseCode == "" {
		return errors.New("course code is required")
	}

	if err := s.courseRepo.Create(ctx, course); err != nil {
		return err
	}

	// Create default section
	section := &models.CourseSection{
		CourseID:      course.ID,
		Name:          course.Name,
		WorkflowState: "active",
	}
	if err := s.sectionRepo.Create(ctx, section); err != nil {
		return err
	}

	// Enroll creator as teacher
	enrollment := &models.Enrollment{
		UserID:          creatorID,
		CourseID:        course.ID,
		CourseSectionID: &section.ID,
		Type:            "TeacherEnrollment",
		Role:            "TeacherEnrollment",
		WorkflowState:   models.EnrollmentActive,
	}
	return s.enrollmentRepo.Create(ctx, enrollment)
}

// GetByID resolves a course by id, scoped to the caller's tenant.
// accountID==0 means "no tenant scope" (internal/background callers
// that have already validated tenant ownership upstream, e.g. the
// gamification wiring or seed scripts). Handler callers MUST pass
// callerAccountID(c) — Wave F.
func (s *CourseService) GetByID(ctx context.Context, id, accountID uint) (*models.Course, error) {
	return s.courseRepo.FindByID(ctx, id, accountID)
}

func (s *CourseService) Update(ctx context.Context, course *models.Course) error {
	return s.courseRepo.Update(ctx, course)
}

func (s *CourseService) Delete(ctx context.Context, id uint) error {
	return s.courseRepo.Delete(ctx, id)
}

func (s *CourseService) List(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	return s.courseRepo.List(ctx, 0, params)
}

func (s *CourseService) ListForUser(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	return s.courseRepo.ListByUserID(ctx, userID, 0, params)
}
