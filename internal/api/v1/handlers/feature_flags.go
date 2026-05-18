package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// FeatureFlagHandler exposes Canvas-compatible feature flag endpoints.
//
//	GET    /accounts/:id/features
//	GET    /accounts/:id/features/:feature
//	PUT    /accounts/:id/features/:feature
//	DELETE /accounts/:id/features/:feature
//	(same shape for /courses/:id/features and /users/self/features)
type FeatureFlagHandler struct {
	flagService    *service.FeatureFlagService
	enrollmentRepo repository.EnrollmentRepository
	userRepo       repository.UserRepository
}

func NewFeatureFlagHandler(
	flagService *service.FeatureFlagService,
	enrollmentRepo repository.EnrollmentRepository,
	userRepo repository.UserRepository,
) *FeatureFlagHandler {
	return &FeatureFlagHandler{
		flagService:    flagService,
		enrollmentRepo: enrollmentRepo,
		userRepo:       userRepo,
	}
}

// ---- Account-scoped routes -------------------------------------------------

func (h *FeatureFlagHandler) ListAccountFeatures(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	flags := h.flagService.ListEffectiveFlags(c.Context(), models.FeatureContextAccount, uint(id), h.isSiteAdmin(c))
	return c.JSON(flags)
}

func (h *FeatureFlagHandler) GetAccountFeature(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	feature := c.Params("feature")
	eff, err := h.flagService.GetEffectiveFlag(c.Context(), feature, models.FeatureContextAccount, uint(id))
	if err != nil {
		return responses.NotFound(c, "feature")
	}
	return c.JSON(eff)
}

func (h *FeatureFlagHandler) SetAccountFeature(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	feature := c.Params("feature")
	state, err := h.parseState(c)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	if err := h.flagService.SetState(
		c.Context(), feature, models.FeatureContextAccount, uint(id), state,
		h.isAdmin(c), false,
	); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	eff, _ := h.flagService.GetEffectiveFlag(c.Context(), feature, models.FeatureContextAccount, uint(id))
	return c.JSON(eff)
}

func (h *FeatureFlagHandler) DeleteAccountFeature(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if !h.isAdmin(c) {
		return responses.Error(c, fiber.StatusForbidden, "admin permission required")
	}
	if err := h.flagService.Reset(c.Context(), c.Params("feature"), models.FeatureContextAccount, uint(id)); err != nil {
		return responses.InternalError(c, "could not reset flag")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- Course-scoped routes --------------------------------------------------

func (h *FeatureFlagHandler) ListCourseFeatures(c *fiber.Ctx) error {
	id, err := h.courseID(c)
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	flags := h.flagService.ListEffectiveFlags(c.Context(), models.FeatureContextCourse, id, h.isSiteAdmin(c))
	return c.JSON(flags)
}

func (h *FeatureFlagHandler) GetCourseFeature(c *fiber.Ctx) error {
	id, err := h.courseID(c)
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	feature := c.Params("feature")
	eff, err := h.flagService.GetEffectiveFlag(c.Context(), feature, models.FeatureContextCourse, id)
	if err != nil {
		return responses.NotFound(c, "feature")
	}
	return c.JSON(eff)
}

func (h *FeatureFlagHandler) SetCourseFeature(c *fiber.Ctx) error {
	id, err := h.courseID(c)
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	state, err := h.parseState(c)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	feature := c.Params("feature")
	if err := h.flagService.SetState(
		c.Context(), feature, models.FeatureContextCourse, id, state,
		h.isAdmin(c), h.isTeacher(c, id),
	); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	eff, _ := h.flagService.GetEffectiveFlag(c.Context(), feature, models.FeatureContextCourse, id)
	return c.JSON(eff)
}

func (h *FeatureFlagHandler) DeleteCourseFeature(c *fiber.Ctx) error {
	id, err := h.courseID(c)
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	if !h.isAdmin(c) && !h.isTeacher(c, id) {
		return responses.Error(c, fiber.StatusForbidden, "teacher or admin permission required")
	}
	if err := h.flagService.Reset(c.Context(), c.Params("feature"), models.FeatureContextCourse, id); err != nil {
		return responses.InternalError(c, "could not reset flag")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- Per-user routes (self) -----------------------------------------------

func (h *FeatureFlagHandler) ListUserFeatures(c *fiber.Ctx) error {
	uid, ok := c.Locals("user_id").(uint)
	if !ok || uid == 0 {
		return responses.Unauthorized(c)
	}
	flags := h.flagService.ListEffectiveFlags(c.Context(), models.FeatureContextUser, uid, h.isSiteAdmin(c))
	return c.JSON(flags)
}

func (h *FeatureFlagHandler) GetUserFeature(c *fiber.Ctx) error {
	uid, ok := c.Locals("user_id").(uint)
	if !ok || uid == 0 {
		return responses.Unauthorized(c)
	}
	eff, err := h.flagService.GetEffectiveFlag(c.Context(), c.Params("feature"), models.FeatureContextUser, uid)
	if err != nil {
		return responses.NotFound(c, "feature")
	}
	return c.JSON(eff)
}

func (h *FeatureFlagHandler) SetUserFeature(c *fiber.Ctx) error {
	uid, ok := c.Locals("user_id").(uint)
	if !ok || uid == 0 {
		return responses.Unauthorized(c)
	}
	state, err := h.parseState(c)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	feature := c.Params("feature")
	if err := h.flagService.SetState(
		c.Context(), feature, models.FeatureContextUser, uid, state,
		h.isAdmin(c), false,
	); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	eff, _ := h.flagService.GetEffectiveFlag(c.Context(), feature, models.FeatureContextUser, uid)
	return c.JSON(eff)
}

func (h *FeatureFlagHandler) DeleteUserFeature(c *fiber.Ctx) error {
	uid, ok := c.Locals("user_id").(uint)
	if !ok || uid == 0 {
		return responses.Unauthorized(c)
	}
	if err := h.flagService.Reset(c.Context(), c.Params("feature"), models.FeatureContextUser, uid); err != nil {
		return responses.InternalError(c, "could not reset flag")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- helpers ---------------------------------------------------------------

func (h *FeatureFlagHandler) parseState(c *fiber.Ctx) (string, error) {
	// Accept either ?state=on or JSON body {"state":"on"} — Canvas allows both.
	state := c.Query("state")
	if state == "" {
		var body struct {
			State string `json:"state"`
		}
		_ = c.BodyParser(&body)
		state = body.State
	}
	if state == "" {
		return "", fiberErr("state is required")
	}
	return state, nil
}

func (h *FeatureFlagHandler) courseID(c *fiber.Ctx) (uint, error) {
	idStr := c.Params("course_id")
	if idStr == "" {
		idStr = c.Params("id")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func (h *FeatureFlagHandler) isAdmin(c *fiber.Ctx) bool {
	if v, ok := c.Locals("is_admin").(bool); ok && v {
		return true
	}
	uid, ok := c.Locals("user_id").(uint)
	if !ok || uid == 0 {
		return false
	}
	user, err := h.userRepo.FindByID(c.Context(), uid)
	if err != nil {
		return false
	}
	return user.Role == "admin"
}

// isSiteAdmin: in single-tenant Paper LMS this is identical to admin.
func (h *FeatureFlagHandler) isSiteAdmin(c *fiber.Ctx) bool {
	return h.isAdmin(c)
}

func (h *FeatureFlagHandler) isTeacher(c *fiber.Ctx, courseID uint) bool {
	uid, ok := c.Locals("user_id").(uint)
	if !ok || uid == 0 {
		return false
	}
	enr, err := h.enrollmentRepo.FindByUserAndCourse(c.Context(), uid, courseID, callerAccountID(c))
	if err != nil {
		return false
	}
	return enr.WorkflowState == "active" &&
		(enr.Type == "TeacherEnrollment" || enr.Type == "TaEnrollment" || enr.Type == "DesignerEnrollment")
}

// fiberErr wraps a string in error so the handler stays single-return.
type fiberError struct{ msg string }

func (e *fiberError) Error() string { return e.msg }
func fiberErr(msg string) error     { return &fiberError{msg: msg} }
