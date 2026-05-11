package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type EnrollmentService struct {
	enrollmentRepo repository.EnrollmentRepository
}

func NewEnrollmentService(enrollmentRepo repository.EnrollmentRepository) *EnrollmentService {
	return &EnrollmentService{enrollmentRepo: enrollmentRepo}
}

var validEnrollmentTypes = map[string]bool{
	"StudentEnrollment":  true,
	"TeacherEnrollment":  true,
	"TaEnrollment":       true,
	"ObserverEnrollment": true,
	"DesignerEnrollment": true,
}

func (s *EnrollmentService) Create(ctx context.Context, enrollment *models.Enrollment) error {
	if !validEnrollmentTypes[enrollment.Type] {
		return errors.New("invalid enrollment type")
	}
	enrollment.Role = enrollment.Type
	if enrollment.WorkflowState == "" {
		enrollment.WorkflowState = "active"
	}

	// Check for existing enrollment
	existing, _ := s.enrollmentRepo.FindByUserAndCourse(ctx, enrollment.UserID, enrollment.CourseID)
	if existing != nil {
		return errors.New("user is already enrolled in this course")
	}

	return s.enrollmentRepo.Create(ctx, enrollment)
}

func (s *EnrollmentService) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	return s.enrollmentRepo.ListByCourseID(ctx, courseID, params)
}

func (s *EnrollmentService) ListByUser(ctx context.Context, userID uint) ([]models.Enrollment, error) {
	return s.enrollmentRepo.ListByUserID(ctx, userID)
}

func (s *EnrollmentService) GetUserRole(ctx context.Context, userID, courseID uint) (string, error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID)
	if err != nil {
		return "", err
	}
	return enrollment.Type, nil
}

func (s *EnrollmentService) CountStudentsByCourseIDs(ctx context.Context, courseIDs []uint) (map[uint]int64, error) {
	return s.enrollmentRepo.CountByCourseIDs(ctx, courseIDs)
}
