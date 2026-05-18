package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// EnrollmentCreatedCallback fires asynchronously after a successful
// EnrollmentService.Create. Mirrors the SubmissionGradedCallback pattern:
// a fresh context.Background() is passed in so the callback survives the
// originating request being cancelled. Gamification rules consume this
// to fire `verb=enrolled, object_type=Course` triggers.
type EnrollmentCreatedCallback func(ctx context.Context, enrollmentID uint)

type EnrollmentService struct {
	enrollmentRepo repository.EnrollmentRepository

	// onCreatedCallbacks fire (in goroutines) after a successful Create.
	// Registered via OnCreated; never invoked in tests unless explicitly
	// wired.
	onCreatedCallbacks []EnrollmentCreatedCallback
}

func NewEnrollmentService(enrollmentRepo repository.EnrollmentRepository) *EnrollmentService {
	return &EnrollmentService{enrollmentRepo: enrollmentRepo}
}

// OnCreated registers a callback to fire after a successful enrollment
// write. The callback runs in a fresh goroutine with a detached context.
// Multiple registrations stack; order is registration order.
func (s *EnrollmentService) OnCreated(cb EnrollmentCreatedCallback) {
	s.onCreatedCallbacks = append(s.onCreatedCallbacks, cb)
}

func (s *EnrollmentService) fireOnCreated(enrollmentID uint) {
	for _, cb := range s.onCreatedCallbacks {
		go func(cb EnrollmentCreatedCallback) {
			defer recoverFromPanic("enrollment OnCreated callback")
			cb(context.Background(), enrollmentID)
		}(cb)
	}
}

var validEnrollmentTypes = map[string]bool{
	"StudentEnrollment":  true,
	"TeacherEnrollment":  true,
	"TaEnrollment":       true,
	"ObserverEnrollment": true,
	"DesignerEnrollment": true,
}

func (s *EnrollmentService) Create(ctx context.Context, enrollment *models.Enrollment, accountID uint) error {
	if !validEnrollmentTypes[enrollment.Type] {
		return errors.New("invalid enrollment type")
	}
	enrollment.Role = enrollment.Type
	if enrollment.WorkflowState == "" {
		enrollment.WorkflowState = "active"
	}

	// Check for existing enrollment
	existing, _ := s.enrollmentRepo.FindByUserAndCourse(ctx, enrollment.UserID, enrollment.CourseID, accountID)
	if existing != nil {
		return errors.New("user is already enrolled in this course")
	}

	if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
		return err
	}

	s.fireOnCreated(enrollment.ID)
	return nil
}

func (s *EnrollmentService) ListByCourse(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	return s.enrollmentRepo.ListByCourseID(ctx, courseID, accountID, params)
}

func (s *EnrollmentService) ListByUser(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error) {
	return s.enrollmentRepo.ListByUserID(ctx, userID, accountID)
}

func (s *EnrollmentService) GetUserRole(ctx context.Context, userID, courseID, accountID uint) (string, error) {
	enrollment, err := s.enrollmentRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	if err != nil {
		return "", err
	}
	return enrollment.Type, nil
}

func (s *EnrollmentService) CountStudentsByCourseIDs(ctx context.Context, courseIDs []uint, accountID uint) (map[uint]int64, error) {
	return s.enrollmentRepo.CountByCourseIDs(ctx, courseIDs, accountID)
}
