package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ResourceAuthorizer provides handler-level authorization checks for standalone
// routes that don't have course-scoped middleware. Handlers call these methods
// after fetching the resource to verify the authenticated user has access.
//
// Contract: every check returns a non-nil error when access is denied and
// does NOT write the response itself. Callers use
//
//	if err := h.authz.RequireCourseInstructor(c, courseID); err != nil {
//		return err
//	}
//
// and the returned *fiber.Error reaches the app ErrorHandler, which renders
// the usual {"errors":[{"message":...}]} envelope with the right status.
// (Before this contract the checks wrote a 403 and returned nil — Fiber's
// Ctx.JSON returns nil on success — so every caller fell through to the
// destructive step: a student could delete an announcement and get a
// "403" whose body said {"delete":true}.)
type ResourceAuthorizer struct {
	enrollmentRepo repository.EnrollmentRepository
	userRepo       repository.UserRepository
	courseRepo     repository.CourseRepository
}

func NewResourceAuthorizer(enrollmentRepo repository.EnrollmentRepository, userRepo repository.UserRepository, courseRepo repository.CourseRepository) *ResourceAuthorizer {
	return &ResourceAuthorizer{enrollmentRepo: enrollmentRepo, userRepo: userRepo, courseRepo: courseRepo}
}

// authzForbidden builds the 403 error returned by the checks below. It does
// not write the response (see the ResourceAuthorizer contract).
func authzForbidden(msg string) error {
	return fiber.NewError(fiber.StatusForbidden, msg)
}

// authzNotFound builds a 404 for a resource outside the caller's tenant
// (existence-leak contract — never 403 across tenants).
func authzNotFound(resource string) error {
	return fiber.NewError(fiber.StatusNotFound, "The specified "+resource+" was not found")
}

// loadCourse loads the course scoped to the caller's tenant. A super_admin
// (role crosses tenants by design) loads it unscoped.
func (a *ResourceAuthorizer) loadCourse(c *fiber.Ctx, role string, courseID uint) (*models.Course, error) {
	if a.courseRepo == nil {
		return nil, authzNotFound("course")
	}
	scope := callerAccountID(c)
	if role == middleware.RoleSuperAdmin {
		scope = 0
	}
	course, err := a.courseRepo.FindByID(c.Context(), courseID, scope)
	if err != nil || course == nil {
		return nil, authzNotFound("course")
	}
	return course, nil
}

func courseIsUnpublished(course *models.Course) bool {
	switch course.WorkflowState {
	case models.CourseClaimed, models.CourseCreated, models.CourseUnpublished:
		return true
	}
	return false
}

// requireCourse is the shared body of RequireCourseInstructor and
// RequireCourseEnrolled. The course must be in the caller's tenant; admins of
// that tenant (and super_admins) pass; everyone else needs an active
// enrollment, of an instructor type when instructorOnly is set. Students and
// observers are refused while the course is unpublished (Canvas parity).
func (a *ResourceAuthorizer) requireCourse(c *fiber.Ctx, courseID uint, instructorOnly bool) error {
	var roleSet map[string]bool
	if instructorOnly {
		roleSet = map[string]bool{"TeacherEnrollment": true, "TaEnrollment": true}
	}
	return a.requireCourseRoles(c, courseID, roleSet)
}

// RequireCourseTeacher checks that the authenticated user is a teacher in
// the given course, or an admin of the course's tenant (section
// management and cross-listing, which Canvas does not grant TAs).
func (a *ResourceAuthorizer) RequireCourseTeacher(c *fiber.Ctx, courseID uint) error {
	return a.requireCourseRoles(c, courseID, map[string]bool{"TeacherEnrollment": true})
}

// requireCourseRoles: roleSet nil = any enrollment type.
func (a *ResourceAuthorizer) requireCourseRoles(c *fiber.Ctx, courseID uint, roleSet map[string]bool) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return authzForbidden("authentication required")
	}
	role := middleware.EffectiveRole(c, a.userRepo)
	course, err := a.loadCourse(c, role, courseID)
	if err != nil {
		return err
	}
	if role == middleware.RoleAdmin || role == middleware.RoleSuperAdmin {
		return nil
	}
	// Date-based access, multiple enrollments and read-only concluded
	// enrollments resolve exactly as in the course gate middleware.
	dec, denial := middleware.ResolveCourseEnrollment(c, a.enrollmentRepo, userID, courseID, callerAccountID(c), course, roleSet)
	if denial != "" {
		if roleSet != nil && dec.RoleMismatch {
			return authzForbidden("instructor access required")
		}
		return authzForbidden("not enrolled in this course")
	}
	enrollment := dec.Enrollment
	if courseIsUnpublished(course) && (enrollment.Type == "StudentEnrollment" || enrollment.Type == "ObserverEnrollment") {
		return authzForbidden("this course has not been published yet")
	}
	if msg := middleware.ReadOnlyDenial(c, dec); msg != "" {
		return authzForbidden(msg)
	}
	return nil
}

// RequireCourseInstructor checks that the authenticated user is an instructor
// (teacher or TA) in the given course, or is an admin of the course's tenant.
func (a *ResourceAuthorizer) RequireCourseInstructor(c *fiber.Ctx, courseID uint) error {
	return a.requireCourse(c, courseID, true)
}

// RequireCourseEnrolled checks that the authenticated user is enrolled in the
// given course (any role), or is an admin of the course's tenant.
func (a *ResourceAuthorizer) RequireCourseEnrolled(c *fiber.Ctx, courseID uint) error {
	return a.requireCourse(c, courseID, false)
}

// RequireOwnerOrAdmin checks that the authenticated user is the resource owner
// or an admin of the owner's tenant (or a super_admin).
func (a *ResourceAuthorizer) RequireOwnerOrAdmin(c *fiber.Ctx, ownerUserID uint) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return authzForbidden("authentication required")
	}
	if userID == ownerUserID {
		return nil
	}
	switch middleware.EffectiveRole(c, a.userRepo) {
	case middleware.RoleSuperAdmin:
		return nil
	case middleware.RoleAdmin:
		if a.userRepo != nil {
			if owner, err := a.userRepo.FindByID(c.Context(), ownerUserID, callerAccountID(c)); err == nil && owner != nil {
				return nil
			}
		}
	}
	return authzForbidden("you can only access your own resources")
}

// RequireUserInTenant returns a 404 unless userID names a user in the
// caller's tenant. A super_admin crosses tenants by design. Use it on
// admin-gated routes keyed by a bare user id (the `admin` middleware only
// checks the tenant when the URL carries :account_id).
func (a *ResourceAuthorizer) RequireUserInTenant(c *fiber.Ctx, userID uint) error {
	if userID == 0 || a.userRepo == nil {
		return authzNotFound("user")
	}
	scope := callerAccountID(c)
	if middleware.EffectiveRole(c, a.userRepo) == middleware.RoleSuperAdmin {
		scope = 0
	}
	if u, err := a.userRepo.FindByID(c.Context(), userID, scope); err != nil || u == nil {
		return authzNotFound("user")
	}
	return nil
}

// RequireCourseInTenant returns a 404 unless courseID names a course in the
// caller's tenant (any course for a super_admin). It checks no enrollment.
func (a *ResourceAuthorizer) RequireCourseInTenant(c *fiber.Ctx, courseID uint) error {
	_, err := a.loadCourse(c, middleware.EffectiveRole(c, a.userRepo), courseID)
	return err
}
