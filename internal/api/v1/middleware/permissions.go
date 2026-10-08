package middleware

import (
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/gofiber/fiber/v2"
)

// Global user roles, ordered by privilege. Kept as constants so the
// masquerade cap (EffectiveRole) can compare them.
const (
	RoleUser       = "user"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// effectiveRoleLocal caches the DB-verified, masquerade-capped role for
// the rest of the request. Only EffectiveRole writes it.
const effectiveRoleLocal = "effective_role"

// RoleRank orders the global roles: user < admin < super_admin.
func RoleRank(role string) int {
	switch role {
	case RoleSuperAdmin:
		return 2
	case RoleAdmin:
		return 1
	default:
		return 0
	}
}

// EffectiveRole returns the caller's global role as the database knows it
// ("user", "admin" or "super_admin"), never the JWT `role` claim, which can
// be stale for up to the token TTL.
//
// During a masquerade the session's user_id is the impersonated user, so the
// impersonated user's role is capped by the REAL user's role (masquerade_by):
// an account admin who masquerades as a super_admin gets at most admin
// powers. If the real user can't be loaded the session degrades to a plain
// user (fail closed).
//
// The result is cached in Locals, and is_admin / is_super_admin Locals are
// overwritten with the DB truth so downstream handlers never act on the JWT
// hint once a permission gate has run.
func EffectiveRole(c *fiber.Ctx, users repository.UserRepository) string {
	if cached, ok := c.Locals(effectiveRoleLocal).(string); ok {
		return cached
	}
	if users == nil {
		// Nothing to verify against and nothing cached by an earlier
		// gate: least privilege, and don't poison the cache.
		return RoleUser
	}
	role := RoleUser
	userID, ok := c.Locals("user_id").(uint)
	if ok && userID != 0 {
		// AUTH-INTERNAL: role lookup for the JWT subject. accountID=0 is
		// correct: the global role is not tenant-keyed, and tenant scope is
		// enforced separately by each gate below.
		if u, err := users.FindByID(c.Context(), userID, 0); err == nil && u != nil {
			role = u.Role
		}
		if realID, ok := c.Locals("masquerade_by").(uint); ok && realID != 0 {
			realRole := RoleUser
			// AUTH-INTERNAL: the masquerader is looked up by id with no
			// tenant scope (their home tenant may differ from the
			// impersonated user's).
			if ru, err := users.FindByID(c.Context(), realID, 0); err == nil && ru != nil {
				realRole = ru.Role
			}
			if RoleRank(realRole) < RoleRank(role) {
				role = realRole
			}
		}
	}
	if RoleRank(role) == 0 {
		role = RoleUser
	}
	c.Locals(effectiveRoleLocal, role)
	c.Locals("is_admin", RoleRank(role) >= 1)
	c.Locals("is_super_admin", role == RoleSuperAdmin)
	return role
}

// PermissionMiddleware provides role-based access control for routes.
type PermissionMiddleware struct {
	enrollmentRepo repository.EnrollmentRepository
	userRepo       repository.UserRepository
	courseRepo     repository.CourseRepository
}

// NewPermissionMiddleware creates a new permission middleware. courseRepo is
// required by the course-scoped gates (RequireCourseRole / RequireEnrolled):
// they load the course tenant-scoped so an account admin's bypass only
// applies inside their own tenant, and so students are kept out of
// unpublished courses.
func NewPermissionMiddleware(enrollmentRepo repository.EnrollmentRepository, userRepo repository.UserRepository, courseRepo repository.CourseRepository) *PermissionMiddleware {
	return &PermissionMiddleware{
		enrollmentRepo: enrollmentRepo,
		userRepo:       userRepo,
		courseRepo:     courseRepo,
	}
}

// forbidden returns a 403 error in Canvas format.
func forbidden(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"errors": []fiber.Map{{"message": msg}},
	})
}

// notFound returns a 404 in Canvas format. Used for cross-tenant misses so
// the response doesn't reveal that the resource exists in another tenant.
func notFound(c *fiber.Ctx, resource string) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"errors": []fiber.Map{{"message": "The specified " + resource + " was not found"}},
	})
}

// urlAccountID returns the account id named in the URL of an
// account-scoped route (/accounts/:account_id/... or /accounts/:id...).
// present=false means the route is not account-scoped.
func urlAccountID(c *fiber.Ctx) (id uint, present bool, valid bool) {
	raw := c.Params("account_id")
	if raw == "" && strings.HasPrefix(c.Path(), "/api/v1/accounts/") {
		raw = c.Params("id")
	}
	if raw == "" {
		return 0, false, false
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0, true, false
	}
	return uint(n), true, true
}

// RequireAdmin ensures the caller is an account admin (or super_admin) AND,
// on account-scoped routes, that the :account_id in the URL is the caller's
// own tenant.
//
// The URL check is the load-bearing part: handlers under
// /accounts/:account_id/... read the account id from the URL, and the old
// tenant check here compared two values that both came from the caller's own
// JWT, so it never rejected anything. An account admin of tenant B could
// therefore read or write tenant A's roles, terms, audit log and feature
// flags by changing the URL. A mismatch is now a 404 (existence-leak
// contract), matching assertSameTenant.
//
// A super_admin (platform operator) passes for any account — the role is
// strictly above account-admin and crosses tenant boundaries by design.
// During a masquerade the role is capped by the real user's role (see
// EffectiveRole).
func (pm *PermissionMiddleware) RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			return forbidden(c, "user not authenticated")
		}

		role := EffectiveRole(c, pm.userRepo)
		if role == RoleSuperAdmin {
			return c.Next()
		}
		if role != RoleAdmin {
			return forbidden(c, "user is not an account admin")
		}

		if urlAcct, present, valid := urlAccountID(c); present {
			if !valid {
				return notFound(c, "account")
			}
			callerAccount, _ := c.Locals("account_id").(uint)
			if callerAccount == 0 || urlAcct != callerAccount {
				return notFound(c, "account")
			}
		}

		return c.Next()
	}
}

// RequireSuperAdmin gates routes that only a platform operator may
// hit. The Super-Admin Settings Engine (Wave 2 onward) uses this for
// every /superadmin/* endpoint. A super_admin's account_id Locals is
// special-cased — assertSameTenant treats this role as authorized for
// any tenant so platform operators can manage settings for any
// account in the deployment.
//
// Distinct from RequireAdmin: an account-admin (role='admin') is NOT
// a super_admin and gets 403 here even on their own tenant. An admin
// masquerading as a super_admin is also refused (EffectiveRole caps the
// impersonated role at the real user's role).
func (pm *PermissionMiddleware) RequireSuperAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			return forbidden(c, "user not authenticated")
		}

		if EffectiveRole(c, pm.userRepo) != RoleSuperAdmin {
			return forbidden(c, "super-admin role required")
		}
		return c.Next()
	}
}

// RequireCourseRole ensures the user has one of the specified enrollment types
// in the course identified by :course_id param. Admins of the course's tenant
// (and super_admins) pass.
// Valid roles: "TeacherEnrollment", "TaEnrollment", "StudentEnrollment",
// "ObserverEnrollment", "DesignerEnrollment"
func (pm *PermissionMiddleware) RequireCourseRole(roles ...string) fiber.Handler {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(c *fiber.Ctx) error {
		return pm.courseGate(c, roleSet, true)
	}
}

// RequireEnrolled ensures the user is enrolled in the course (any role).
// Admins of the course's tenant (and super_admins) pass.
func (pm *PermissionMiddleware) RequireEnrolled() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return pm.courseGate(c, nil, false)
	}
}

// courseUnpublished reports whether a course is still being built. Canvas
// denies student and observer reads until the course is published
// (available); a concluded (completed) course stays readable.
func courseUnpublished(course *models.Course) bool {
	switch course.WorkflowState {
	case models.CourseClaimed, models.CourseCreated, models.CourseUnpublished:
		return true
	}
	return false
}

// learnerEnrollment reports whether the enrollment type is a learner-side
// role that must not see an unpublished course.
func learnerEnrollment(enrollmentType string) bool {
	return enrollmentType == "StudentEnrollment" || enrollmentType == "ObserverEnrollment"
}

// courseGate is the shared body of RequireCourseRole and RequireEnrolled.
// roleSet == nil means "any enrollment type".
//
//   - The course is loaded scoped to the caller's tenant (super_admin: no
//     scope). A course outside the tenant is a 404, for admins too — the
//     admin bypass used to apply to every tenant's courses.
//   - Admins of that tenant and super_admins bypass the enrollment check.
//   - Everyone else needs an active enrollment of an allowed type, and
//     students / observers are refused while the course is unpublished.
func (pm *PermissionMiddleware) courseGate(c *fiber.Ctx, roleSet map[string]bool, adminActsAsTeacher bool) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return forbidden(c, "user not authenticated")
	}

	role := EffectiveRole(c, pm.userRepo)
	isAdmin := RoleRank(role) >= 1

	courseID, err := pm.extractCourseID(c)
	if err != nil {
		// Route carries no course id. Preserve the historical contract:
		// only admins pass (the handler is responsible for tenant scope).
		if isAdmin {
			if adminActsAsTeacher {
				c.Locals("enrollment_type", "TeacherEnrollment")
			}
			return c.Next()
		}
		return forbidden(c, "invalid course_id")
	}

	callerAccount, _ := c.Locals("account_id").(uint)
	scope := callerAccount
	if role == RoleSuperAdmin {
		scope = 0 // platform operator: crosses tenants by design
	} else if callerAccount == 0 {
		return notFound(c, "course")
	}

	var course *models.Course
	if pm.courseRepo != nil {
		course, err = pm.courseRepo.FindByID(c.Context(), courseID, scope)
		if err != nil || course == nil {
			return notFound(c, "course")
		}
	} else if isAdmin {
		// Without a course repo the admin bypass can't be tenant-checked:
		// fail closed. (Non-admins still go through the tenant-scoped
		// enrollment lookup below.)
		return notFound(c, "course")
	}

	if isAdmin {
		// The course was found inside the caller's tenant (or the caller
		// is a super_admin), so the admin bypass is tenant-scoped.
		if adminActsAsTeacher {
			c.Locals("enrollment_type", "TeacherEnrollment") // admin gets teacher-level access
		}
		return c.Next()
	}

	enrollment, err := pm.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccount)
	if err != nil || enrollment == nil {
		return forbidden(c, "user is not enrolled in this course")
	}

	if enrollment.WorkflowState != "active" {
		return forbidden(c, "enrollment is not active")
	}

	if roleSet != nil && !roleSet[enrollment.Type] {
		return forbidden(c, "insufficient permissions for this action")
	}

	if course != nil && courseUnpublished(course) && learnerEnrollment(enrollment.Type) {
		return forbidden(c, "this course has not been published yet")
	}

	c.Locals("enrollment_type", enrollment.Type)
	c.Locals("enrollment_id", enrollment.ID)
	return c.Next()
}

// RequireSelfOrAdmin ensures the user_id in the URL matches the authenticated
// user, or the caller is an admin of the target user's tenant (or a
// super_admin).
func (pm *PermissionMiddleware) RequireSelfOrAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			return forbidden(c, "user not authenticated")
		}

		// Check if the URL user_id matches or is "self"
		// Routes may use :user_id or :id — check both
		paramUserID := c.Params("user_id")
		if paramUserID == "" {
			paramUserID = c.Params("id")
		}
		if paramUserID == "self" {
			return c.Next()
		}
		if paramUserID == "" {
			return forbidden(c, "missing user identifier in URL")
		}

		targetID, err := strconv.ParseUint(paramUserID, 10, 64)
		if err != nil {
			return forbidden(c, "invalid user_id")
		}

		if uint(targetID) == userID {
			return c.Next()
		}

		// Not self — must be an admin who can see the target user.
		switch EffectiveRole(c, pm.userRepo) {
		case RoleSuperAdmin:
			return c.Next()
		case RoleAdmin:
			callerAccount, _ := c.Locals("account_id").(uint)
			if callerAccount != 0 {
				if target, err := pm.userRepo.FindByID(c.Context(), uint(targetID), callerAccount); err == nil && target != nil {
					return c.Next()
				}
			}
		}

		return forbidden(c, "can only access your own data unless admin")
	}
}

// RequireInstructor is a convenience for RequireCourseRole with teacher/TA roles.
func (pm *PermissionMiddleware) RequireInstructor() fiber.Handler {
	return pm.RequireCourseRole("TeacherEnrollment", "TaEnrollment")
}

// RequireStudentOrHigher allows students, TAs, teachers, designers, and admins.
func (pm *PermissionMiddleware) RequireStudentOrHigher() fiber.Handler {
	return pm.RequireCourseRole("StudentEnrollment", "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment", "ObserverEnrollment")
}

// extractCourseID gets the course ID from various URL param names.
func (pm *PermissionMiddleware) extractCourseID(c *fiber.Ctx) (uint, error) {
	// Try :course_id first, then :id for /courses/:id routes
	courseIDStr := c.Params("course_id")
	if courseIDStr == "" {
		// For routes like /courses/:id, check the path
		path := c.Path()
		if strings.HasPrefix(path, "/api/v1/courses/") {
			courseIDStr = c.Params("id")
		}
	}

	if courseIDStr == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "no course_id found")
	}

	id, err := strconv.ParseUint(courseIDStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return uint(id), nil
}
