package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ResourceAuthorizer provides handler-level authorization checks for standalone
// routes that don't have course-scoped middleware. Handlers call these methods
// after fetching the resource to verify the authenticated user has access.
type ResourceAuthorizer struct {
	enrollmentRepo repository.EnrollmentRepository
	userRepo       repository.UserRepository
}

func NewResourceAuthorizer(enrollmentRepo repository.EnrollmentRepository, userRepo repository.UserRepository) *ResourceAuthorizer {
	return &ResourceAuthorizer{enrollmentRepo: enrollmentRepo, userRepo: userRepo}
}

func authzForbidden(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"errors": []fiber.Map{{"message": msg}},
	})
}

func (a *ResourceAuthorizer) isAdmin(c *fiber.Ctx) bool {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return false
	}
	// AUTH-INTERNAL: cached role check. userID is the JWT subject;
	// role is tenant-independent. accountID=0 is correct.
	user, err := a.userRepo.FindByID(c.Context(), userID, 0)
	return err == nil && user.Role == "admin"
}

// RequireCourseInstructor checks that the authenticated user is an instructor
// (teacher or TA) in the given course, or is an admin.
func (a *ResourceAuthorizer) RequireCourseInstructor(c *fiber.Ctx, courseID uint) error {
	if a.isAdmin(c) {
		return nil
	}
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return authzForbidden(c, "authentication required")
	}
	enrollment, err := a.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccountID(c))
	if err != nil || enrollment.WorkflowState != "active" {
		return authzForbidden(c, "not enrolled in this course")
	}
	if enrollment.Type != "TeacherEnrollment" && enrollment.Type != "TaEnrollment" {
		return authzForbidden(c, "instructor access required")
	}
	return nil
}

// RequireCourseEnrolled checks that the authenticated user is enrolled in the
// given course (any role), or is an admin.
func (a *ResourceAuthorizer) RequireCourseEnrolled(c *fiber.Ctx, courseID uint) error {
	if a.isAdmin(c) {
		return nil
	}
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return authzForbidden(c, "authentication required")
	}
	enrollment, err := a.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccountID(c))
	if err != nil || enrollment.WorkflowState != "active" {
		return authzForbidden(c, "not enrolled in this course")
	}
	return nil
}

// RequireOwnerOrAdmin checks that the authenticated user is the resource owner
// or an admin.
func (a *ResourceAuthorizer) RequireOwnerOrAdmin(c *fiber.Ctx, ownerUserID uint) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return authzForbidden(c, "authentication required")
	}
	if userID == ownerUserID {
		return nil
	}
	if a.isAdmin(c) {
		return nil
	}
	return authzForbidden(c, "you can only access your own resources")
}
