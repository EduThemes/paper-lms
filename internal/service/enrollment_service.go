package service

import (
	"context"
	"errors"
	"strings"

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

	// Tenant guards for Create (B068): the enrolled user must belong to
	// the course's account and any section must belong to the course.
	// Wired via SetTenantRepos; Create fails closed when they are unset.
	userRepo    repository.UserRepository
	courseRepo  repository.CourseRepository
	sectionRepo repository.SectionRepository

	// roleResolver validates a custom role given to an enrollment (G102,
	// enrollment_custom_role.go). Wired via SetRoleResolver.
	roleResolver EnrollmentRoleResolver

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

// SetTenantRepos wires the repositories Create uses to keep an
// enrollment inside one tenant. Returns the service for chaining.
func (s *EnrollmentService) SetTenantRepos(userRepo repository.UserRepository, courseRepo repository.CourseRepository, sectionRepo repository.SectionRepository) *EnrollmentService {
	s.userRepo = userRepo
	s.courseRepo = courseRepo
	s.sectionRepo = sectionRepo
	return s
}

// ErrEnrollmentUserNotFound / ErrEnrollmentSectionNotFound are returned
// when the target user is not in the course's tenant, or the section is
// not in the course. Both are surfaced as a generic 404/400 so a teacher
// can't probe other tenants' user ids.
var (
	ErrEnrollmentUserNotFound    = errors.New("user not found")
	ErrEnrollmentSectionNotFound = errors.New("course section not found")
)

// validateTarget ties the enrollment's user and section to the course.
// accountID is the caller's tenant, used to scope the course lookup
// (0 = already validated upstream, e.g. a super_admin request).
func (s *EnrollmentService) validateTarget(ctx context.Context, enrollment *models.Enrollment, accountID uint) error {
	if s.userRepo == nil || s.courseRepo == nil || s.sectionRepo == nil {
		return errors.New("enrollment tenant guards not configured")
	}
	course, err := s.courseRepo.FindByID(ctx, enrollment.CourseID, accountID)
	if err != nil || course == nil {
		return errors.New("course not found")
	}
	user, err := s.userRepo.FindByID(ctx, enrollment.UserID, course.AccountID)
	if err != nil || user == nil || user.AccountID != course.AccountID {
		return ErrEnrollmentUserNotFound
	}
	if enrollment.CourseSectionID != nil {
		section, err := s.sectionRepo.FindByID(ctx, *enrollment.CourseSectionID)
		if err != nil || section == nil || section.CourseID != enrollment.CourseID {
			return ErrEnrollmentSectionNotFound
		}
	}
	return nil
}

func (s *EnrollmentService) Create(ctx context.Context, enrollment *models.Enrollment, accountID uint) error {
	if !validEnrollmentTypes[enrollment.Type] {
		return errors.New("invalid enrollment type")
	}
	if err := s.validateTarget(ctx, enrollment, accountID); err != nil {
		return err
	}
	if err := s.applyCustomRole(ctx, enrollment); err != nil {
		return err
	}
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

// FindEnrollableUsers resolves an identifier typed into a course's "Add
// People" form to users of the course's tenant (B073). Like Canvas's Add
// People dialog it matches exactly — email, login ID or SIS user ID,
// case-insensitive — rather than by name substring, so course staff can
// add a person they can identify without being able to browse the
// tenant's user directory. accountID scopes the course lookup (0 for a
// super_admin).
func (s *EnrollmentService) FindEnrollableUsers(ctx context.Context, courseID uint, term string, accountID uint) ([]models.User, error) {
	if s.userRepo == nil || s.courseRepo == nil {
		return nil, errors.New("enrollment tenant guards not configured")
	}
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, nil
	}
	course, err := s.courseRepo.FindByID(ctx, courseID, accountID)
	if err != nil || course == nil {
		return nil, errors.New("course not found")
	}

	seen := map[uint]bool{}
	var out []models.User
	add := func(u *models.User) {
		if u == nil || u.AccountID != course.AccountID || seen[u.ID] {
			return
		}
		seen[u.ID] = true
		out = append(out, *u)
	}

	if u, err := s.userRepo.FindByLoginID(ctx, term); err == nil {
		add(u)
	}
	if u, err := s.userRepo.FindBySISUserID(ctx, term); err == nil {
		add(u)
	}
	// Email: the tenant-scoped search narrows the candidates, then only
	// exact (case-insensitive) email or login matches are kept.
	if res, err := s.userRepo.Search(ctx, term, course.AccountID, repository.PaginationParams{Page: 1, PerPage: 50}); err == nil && res != nil {
		for i := range res.Items {
			u := &res.Items[i]
			if strings.EqualFold(u.Email, term) || strings.EqualFold(u.LoginID, term) {
				add(u)
			}
		}
	}
	return out, nil
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
