package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// EnrollmentLifecycleService owns everything that changes an existing
// enrollment (G105 edit/conclude/deactivate/delete/reactivate, G122
// invitation accept/reject) plus the duplicate rule that allows several
// enrollments per user per course (G110).
//
// Canvas semantics:
//   - conclude   active|inactive -> completed (completed_at stamped);
//     the user keeps read-only access.
//   - deactivate active|completed -> inactive; no access, still listed.
//   - delete     any live state  -> deleted; gone from the roster.
//   - reactivate inactive|completed -> active (completed_at cleared).
//   - accept     invited -> active, by the invited user only.
//   - reject     invited -> rejected, by the invited user only.
type EnrollmentLifecycleService struct {
	repo     repository.EnrollmentAdminRepository
	sections repository.SectionRepository
	notifier Notifier
	now      func() time.Time
}

func NewEnrollmentLifecycleService(repo repository.EnrollmentAdminRepository, sections repository.SectionRepository) *EnrollmentLifecycleService {
	return &EnrollmentLifecycleService{repo: repo, sections: sections, now: time.Now}
}

// SetNotifier wires course-invitation notifications through the normal
// notification queue (in-app feed plus the invitee's delivery channels,
// following their notification preferences). Optional.
func (s *EnrollmentLifecycleService) SetNotifier(n Notifier) *EnrollmentLifecycleService {
	s.notifier = n
	return s
}

var (
	ErrEnrollmentNotFound    = errors.New("enrollment not found")
	ErrEnrollmentTransition  = errors.New("that change is not allowed from the enrollment's current state")
	ErrEnrollmentDuplicate   = errors.New("the user already has this role in this section")
	ErrEnrollmentInvalidTask = errors.New("task must be one of conclude, delete, deactivate, inactivate")
	ErrEnrollmentDates       = errors.New("end_at must be after start_at")
	ErrEnrollmentNotInvitee  = errors.New("only the invited user can respond to an invitation")
)

// Enrollment lifecycle tasks (Canvas DELETE /enrollments/:id?task=).
const (
	EnrollmentTaskConclude   = "conclude"
	EnrollmentTaskDelete     = "delete"
	EnrollmentTaskDeactivate = "deactivate"
	EnrollmentTaskInactivate = "inactivate"
)

// enrollmentTransitions is the lifecycle state machine: from -> allowed to.
var enrollmentTransitions = map[models.EnrollmentWorkflow][]models.EnrollmentWorkflow{
	models.EnrollmentActive:    {models.EnrollmentCompleted, models.EnrollmentInactive, models.EnrollmentDeleted},
	models.EnrollmentInactive:  {models.EnrollmentActive, models.EnrollmentCompleted, models.EnrollmentDeleted},
	models.EnrollmentCompleted: {models.EnrollmentActive, models.EnrollmentInactive, models.EnrollmentDeleted},
	models.EnrollmentInvited:   {models.EnrollmentActive, models.EnrollmentRejected, models.EnrollmentDeleted},
	models.EnrollmentRejected:  {models.EnrollmentDeleted},
}

// CanTransitionEnrollment reports whether from -> to is a legal move.
func CanTransitionEnrollment(from, to models.EnrollmentWorkflow) bool {
	if from == "" {
		from = models.EnrollmentActive
	}
	for _, s := range enrollmentTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// FindInCourse loads a live (non-deleted) enrollment of the course.
func (s *EnrollmentLifecycleService) FindInCourse(ctx context.Context, courseID, id, accountID uint) (*models.Enrollment, error) {
	e, err := s.repo.FindInCourse(ctx, id, courseID, accountID)
	if err != nil || e == nil || e.WorkflowState == models.EnrollmentDeleted {
		return nil, ErrEnrollmentNotFound
	}
	return e, nil
}

// EnrollmentUpdate is a partial edit. A *Set flag distinguishes "leave
// as is" from "set to nil" for the nullable fields.
type EnrollmentUpdate struct {
	Type            *string
	SectionSet      bool
	CourseSectionID *uint
	StartAtSet      bool
	StartAt         *time.Time
	EndAtSet        bool
	EndAt           *time.Time
	LimitPrivileges *bool
}

// Update edits role (type), section, dates and the section-limit flag.
func (s *EnrollmentLifecycleService) Update(ctx context.Context, e *models.Enrollment, upd EnrollmentUpdate) error {
	if upd.Type != nil {
		if !validEnrollmentTypes[*upd.Type] {
			return errors.New("invalid enrollment type")
		}
		if *upd.Type != e.Type {
			e.Type = *upd.Type
			e.Role = *upd.Type
		}
	}
	if upd.SectionSet {
		if upd.CourseSectionID != nil {
			if err := s.sectionInCourse(ctx, *upd.CourseSectionID, e.CourseID); err != nil {
				return err
			}
		}
		e.CourseSectionID = upd.CourseSectionID
	}
	if upd.StartAtSet {
		e.StartAt = upd.StartAt
	}
	if upd.EndAtSet {
		e.EndAt = upd.EndAt
	}
	if e.StartAt != nil && e.EndAt != nil && e.EndAt.Before(*e.StartAt) {
		return ErrEnrollmentDates
	}
	if upd.LimitPrivileges != nil {
		e.LimitPrivilegesToCourseSection = *upd.LimitPrivileges
	}
	if err := s.CheckDuplicate(ctx, e, 0); err != nil {
		return err
	}
	return s.repo.Save(ctx, e)
}

func (s *EnrollmentLifecycleService) sectionInCourse(ctx context.Context, sectionID, courseID uint) error {
	if s.sections == nil {
		return ErrEnrollmentSectionNotFound
	}
	section, err := s.sections.FindByID(ctx, sectionID)
	if err != nil || section == nil || section.CourseID != courseID || section.WorkflowState == "deleted" {
		return ErrEnrollmentSectionNotFound
	}
	return nil
}

func sameSection(a, b *uint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// CheckDuplicate enforces Canvas's uniqueness rule for a user's
// enrollments in one course: one live row per (type, section, observed
// user). A user may otherwise hold several enrollments — a TA in one
// section and a student in another. accountID scopes the lookup (0 when
// the course was already validated).
func (s *EnrollmentLifecycleService) CheckDuplicate(ctx context.Context, e *models.Enrollment, accountID uint) error {
	existing, err := s.repo.ListForUserInCourse(ctx, e.UserID, e.CourseID, accountID)
	if err != nil {
		return err
	}
	for i := range existing {
		other := &existing[i]
		if other.ID == e.ID || other.WorkflowState == models.EnrollmentRejected {
			continue
		}
		if other.Type == e.Type && sameSection(other.CourseSectionID, e.CourseSectionID) && sameSection(other.AssociatedUserID, e.AssociatedUserID) {
			return ErrEnrollmentDuplicate
		}
	}
	return nil
}

// ApplyTask concludes, deactivates or deletes an enrollment.
func (s *EnrollmentLifecycleService) ApplyTask(ctx context.Context, e *models.Enrollment, task string) error {
	var to models.EnrollmentWorkflow
	switch task {
	case EnrollmentTaskConclude:
		to = models.EnrollmentCompleted
	case EnrollmentTaskDelete:
		to = models.EnrollmentDeleted
	case EnrollmentTaskDeactivate, EnrollmentTaskInactivate:
		to = models.EnrollmentInactive
	default:
		return ErrEnrollmentInvalidTask
	}
	if !CanTransitionEnrollment(e.WorkflowState, to) {
		return ErrEnrollmentTransition
	}
	e.WorkflowState = to
	if to == models.EnrollmentCompleted {
		now := s.now()
		e.CompletedAt = &now
	}
	return s.repo.Save(ctx, e)
}

// Reactivate restores an inactive or concluded enrollment.
func (s *EnrollmentLifecycleService) Reactivate(ctx context.Context, e *models.Enrollment) error {
	if e.WorkflowState != models.EnrollmentInactive && e.WorkflowState != models.EnrollmentCompleted {
		return ErrEnrollmentTransition
	}
	e.WorkflowState = models.EnrollmentActive
	e.CompletedAt = nil
	return s.repo.Save(ctx, e)
}

// RespondToInvitation accepts (or rejects) the caller's own invitation.
func (s *EnrollmentLifecycleService) RespondToInvitation(ctx context.Context, e *models.Enrollment, callerID uint, accept bool) error {
	if e.UserID != callerID {
		return ErrEnrollmentNotInvitee
	}
	if e.WorkflowState != models.EnrollmentInvited {
		return ErrEnrollmentTransition
	}
	if accept {
		e.WorkflowState = models.EnrollmentActive
	} else {
		e.WorkflowState = models.EnrollmentRejected
	}
	return s.repo.Save(ctx, e)
}

// ListRoster lists a course's enrollments with state / section filters.
func (s *EnrollmentLifecycleService) ListRoster(ctx context.Context, courseID, accountID uint, f repository.EnrollmentListFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	return s.repo.ListByCourseFiltered(ctx, courseID, accountID, f, params)
}

// ListInvitations returns the user's pending course invitations.
func (s *EnrollmentLifecycleService) ListInvitations(ctx context.Context, userID, accountID uint) ([]models.Enrollment, error) {
	return s.repo.ListInvitedForUser(ctx, userID, accountID)
}

// NotifyInvited queues the "you've been invited" notification (Canvas
// "Enrollment Invitation") for an enrollment created in the invited state:
// the in-app notification plus an email (or other channel) delivery that
// honors the invitee's preferences for the Invitation category. It runs off
// the request path on a copy with a detached context, so a slow mail queue
// never holds the enrolling request open; failures never fail the
// enrollment.
func (s *EnrollmentLifecycleService) NotifyInvited(e *models.Enrollment, courseName string) {
	if s.notifier == nil || e == nil || e.UserID == 0 {
		return
	}
	snapshot := *e
	go func() {
		defer recoverFromPanic("enrollment notifyInvited")
		s.notifyInvited(context.Background(), &snapshot, courseName)
	}()
}

func (s *EnrollmentLifecycleService) notifyInvited(ctx context.Context, e *models.Enrollment, courseName string) {
	role := enrollmentRoleLabel(e.Type)
	subject := fmt.Sprintf("Course invitation: %s", courseName)
	body := fmt.Sprintf("<p>You have been invited to join <strong>%s</strong> as a %s.</p><p>Accept or decline the invitation from your dashboard.</p>",
		html.EscapeString(courseName), role)
	_ = s.notifier.QueueNotification(ctx, e.UserID, NotificationEnrollmentInvitation, subject, body, "Course", e.CourseID)
}

func enrollmentRoleLabel(t string) string {
	switch t {
	case models.TeacherEnrollmentType:
		return "teacher"
	case models.TaEnrollmentType:
		return "TA"
	case models.DesignerEnrollmentType:
		return "designer"
	case models.ObserverEnrollmentType:
		return "observer"
	}
	return "student"
}
