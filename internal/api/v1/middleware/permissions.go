package middleware

import (
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/gofiber/fiber/v2"
)

// PermissionMiddleware provides role-based access control for routes.
type PermissionMiddleware struct {
	enrollmentRepo repository.EnrollmentRepository
	userRepo       repository.UserRepository
}

// NewPermissionMiddleware creates a new permission middleware.
func NewPermissionMiddleware(enrollmentRepo repository.EnrollmentRepository, userRepo repository.UserRepository) *PermissionMiddleware {
	return &PermissionMiddleware{
		enrollmentRepo: enrollmentRepo,
		userRepo:       userRepo,
	}
}

// forbidden returns a 403 error in Canvas format.
func forbidden(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"errors": []fiber.Map{{"message": msg}},
	})
}

// RequireAdmin ensures the user has admin role AND is admin of the
// caller's tenant. 13.1.F: after role==admin, also verify
// user.AccountID == c.Locals("account_id"). A site admin (admin
// account_id is the root account) still passes for child accounts via
// the accounts parent-chain traversal — document explicitly when
// account hierarchy is wired (Phase 14).
//
// A super_admin (platform operator) also passes — the role is strictly
// above account-admin and crosses tenant boundaries by design.
func (pm *PermissionMiddleware) RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			return forbidden(c, "user not authenticated")
		}

		// AUTH-INTERNAL: role gate runs as part of the authorization
		// pipeline. accountID=0 is correct — the tenant check happens
		// explicitly below (callerAccount vs user.AccountID), and the
		// user-id is the JWT subject.
		user, err := pm.userRepo.FindByID(c.Context(), userID, 0)
		if err != nil {
			return forbidden(c, "user not found")
		}

		if user.Role == "super_admin" {
			c.Locals("is_admin", true)
			c.Locals("is_super_admin", true)
			return c.Next()
		}

		if user.Role != "admin" {
			return forbidden(c, "user is not an account admin")
		}

		// 13.1.F — tenant scope check. The auth middleware (13.1.B)
		// has populated account_id Locals; this admin must own the
		// resource's tenant. Site admins (account_id == 1, the root)
		// retain access across the deployment until the explicit
		// account-hierarchy traversal lands.
		callerAccount, _ := c.Locals("account_id").(uint)
		if callerAccount != 0 && user.AccountID != callerAccount && user.AccountID != 1 {
			return forbidden(c, "admin role does not extend to this tenant")
		}

		c.Locals("is_admin", true)
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
// a super_admin and gets 403 here even on their own tenant.
func (pm *PermissionMiddleware) RequireSuperAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			return forbidden(c, "user not authenticated")
		}

		// AUTH-INTERNAL: super-admin gate. accountID=0 is required —
		// super_admin role crosses tenant boundaries by definition,
		// and the user-id is the JWT subject.
		user, err := pm.userRepo.FindByID(c.Context(), userID, 0)
		if err != nil {
			return forbidden(c, "user not found")
		}

		if user.Role != "super_admin" {
			return forbidden(c, "super-admin role required")
		}

		c.Locals("is_admin", true)
		c.Locals("is_super_admin", true)
		return c.Next()
	}
}

// RequireCourseRole ensures the user has one of the specified enrollment types
// in the course identified by :course_id param. Admins always pass.
// Valid roles: "TeacherEnrollment", "TaEnrollment", "StudentEnrollment",
// "ObserverEnrollment", "DesignerEnrollment"
func (pm *PermissionMiddleware) RequireCourseRole(roles ...string) fiber.Handler {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			return forbidden(c, "user not authenticated")
		}

		// Admins bypass course role checks
		if pm.isAdmin(c, userID) {
			c.Locals("is_admin", true)
			c.Locals("enrollment_type", "TeacherEnrollment") // admin gets teacher-level access
			return c.Next()
		}

		courseID, err := pm.extractCourseID(c)
		if err != nil {
			return forbidden(c, "invalid course_id")
		}

		callerAccount, _ := c.Locals("account_id").(uint)
		enrollment, err := pm.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccount)
		if err != nil {
			return forbidden(c, "user is not enrolled in this course")
		}

		if enrollment.WorkflowState != "active" {
			return forbidden(c, "enrollment is not active")
		}

		if !roleSet[enrollment.Type] {
			return forbidden(c, "insufficient permissions for this action")
		}

		c.Locals("enrollment_type", enrollment.Type)
		c.Locals("enrollment_id", enrollment.ID)
		return c.Next()
	}
}

// RequireEnrolled ensures the user is enrolled in the course (any role). Admins pass.
func (pm *PermissionMiddleware) RequireEnrolled() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("user_id").(uint)
		if !ok || userID == 0 {
			return forbidden(c, "user not authenticated")
		}

		if pm.isAdmin(c, userID) {
			c.Locals("is_admin", true)
			return c.Next()
		}

		courseID, err := pm.extractCourseID(c)
		if err != nil {
			return forbidden(c, "invalid course_id")
		}

		callerAccount, _ := c.Locals("account_id").(uint)
		enrollment, err := pm.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccount)
		if err != nil {
			return forbidden(c, "user is not enrolled in this course")
		}

		if enrollment.WorkflowState != "active" {
			return forbidden(c, "enrollment is not active")
		}

		c.Locals("enrollment_type", enrollment.Type)
		c.Locals("enrollment_id", enrollment.ID)
		return c.Next()
	}
}

// RequireSelfOrAdmin ensures the user_id in the URL matches the authenticated user,
// or the user is an admin.
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

		// Not self — must be admin
		if pm.isAdmin(c, userID) {
			c.Locals("is_admin", true)
			return c.Next()
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

// isAdmin checks if the user has admin role (with caching in Locals).
func (pm *PermissionMiddleware) isAdmin(c *fiber.Ctx, userID uint) bool {
	if cached, ok := c.Locals("is_admin").(bool); ok {
		return cached
	}

	// AUTH-INTERNAL: cached role lookup. accountID=0 is correct;
	// userID is the JWT subject and role is tenant-independent.
	user, err := pm.userRepo.FindByID(c.Context(), userID, 0)
	if err != nil {
		return false
	}

	isAdmin := user.Role == "admin" || user.Role == "super_admin"
	c.Locals("is_admin", isAdmin)
	if user.Role == "super_admin" {
		c.Locals("is_super_admin", true)
	}
	return isAdmin
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
