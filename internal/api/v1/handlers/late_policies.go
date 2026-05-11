package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type LatePolicyHandler struct {
	latePolicyService *service.LatePolicyService
}

func NewLatePolicyHandler(latePolicyService *service.LatePolicyService) *LatePolicyHandler {
	return &LatePolicyHandler{latePolicyService: latePolicyService}
}

func latePolicyToJSON(p *models.LatePolicy) fiber.Map {
	return fiber.Map{
		"id":                                    p.ID,
		"course_id":                             p.CourseID,
		"missing_submission_deduction_enabled":   p.MissingSubmissionDeductionEnabled,
		"missing_submission_deduction":           p.MissingSubmissionDeduction,
		"late_submission_deduction_enabled":      p.LateSubmissionDeductionEnabled,
		"late_submission_deduction":              p.LateSubmissionDeduction,
		"late_submission_interval":               p.LateSubmissionInterval,
		"late_submission_minimum_percent_enabled": p.LateSubmissionMinimumPercentEnabled,
		"late_submission_minimum_percent":        p.LateSubmissionMinimumPercent,
		"created_at":                            p.CreatedAt,
		"updated_at":                            p.UpdatedAt,
	}
}

func (h *LatePolicyHandler) GetLatePolicy(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	policy, err := h.latePolicyService.GetByCourse(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "late policy")
	}

	return c.JSON(fiber.Map{"late_policy": latePolicyToJSON(policy)})
}

func (h *LatePolicyHandler) CreateLatePolicy(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		LatePolicy struct {
			MissingSubmissionDeductionEnabled   *bool    `json:"missing_submission_deduction_enabled"`
			MissingSubmissionDeduction          *float64 `json:"missing_submission_deduction"`
			LateSubmissionDeductionEnabled      *bool    `json:"late_submission_deduction_enabled"`
			LateSubmissionDeduction             *float64 `json:"late_submission_deduction"`
			LateSubmissionInterval              *string  `json:"late_submission_interval"`
			LateSubmissionMinimumPercentEnabled *bool    `json:"late_submission_minimum_percent_enabled"`
			LateSubmissionMinimumPercent        *float64 `json:"late_submission_minimum_percent"`
		} `json:"late_policy"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	policy := &models.LatePolicy{
		CourseID: uint(courseID),
	}

	if input.LatePolicy.MissingSubmissionDeductionEnabled != nil {
		policy.MissingSubmissionDeductionEnabled = *input.LatePolicy.MissingSubmissionDeductionEnabled
	}
	if input.LatePolicy.MissingSubmissionDeduction != nil {
		policy.MissingSubmissionDeduction = *input.LatePolicy.MissingSubmissionDeduction
	}
	if input.LatePolicy.LateSubmissionDeductionEnabled != nil {
		policy.LateSubmissionDeductionEnabled = *input.LatePolicy.LateSubmissionDeductionEnabled
	}
	if input.LatePolicy.LateSubmissionDeduction != nil {
		policy.LateSubmissionDeduction = *input.LatePolicy.LateSubmissionDeduction
	}
	if input.LatePolicy.LateSubmissionInterval != nil {
		policy.LateSubmissionInterval = *input.LatePolicy.LateSubmissionInterval
	}
	if input.LatePolicy.LateSubmissionMinimumPercentEnabled != nil {
		policy.LateSubmissionMinimumPercentEnabled = *input.LatePolicy.LateSubmissionMinimumPercentEnabled
	}
	if input.LatePolicy.LateSubmissionMinimumPercent != nil {
		policy.LateSubmissionMinimumPercent = *input.LatePolicy.LateSubmissionMinimumPercent
	}

	if err := h.latePolicyService.Create(c.Context(), policy); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"late_policy": latePolicyToJSON(policy)})
}

func (h *LatePolicyHandler) UpdateLatePolicy(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	policy, err := h.latePolicyService.GetByCourse(c.Context(), uint(courseID))
	if err != nil {
		return responses.NotFound(c, "late policy")
	}

	var input struct {
		LatePolicy struct {
			MissingSubmissionDeductionEnabled   *bool    `json:"missing_submission_deduction_enabled"`
			MissingSubmissionDeduction          *float64 `json:"missing_submission_deduction"`
			LateSubmissionDeductionEnabled      *bool    `json:"late_submission_deduction_enabled"`
			LateSubmissionDeduction             *float64 `json:"late_submission_deduction"`
			LateSubmissionInterval              *string  `json:"late_submission_interval"`
			LateSubmissionMinimumPercentEnabled *bool    `json:"late_submission_minimum_percent_enabled"`
			LateSubmissionMinimumPercent        *float64 `json:"late_submission_minimum_percent"`
		} `json:"late_policy"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.LatePolicy.MissingSubmissionDeductionEnabled != nil {
		policy.MissingSubmissionDeductionEnabled = *input.LatePolicy.MissingSubmissionDeductionEnabled
	}
	if input.LatePolicy.MissingSubmissionDeduction != nil {
		policy.MissingSubmissionDeduction = *input.LatePolicy.MissingSubmissionDeduction
	}
	if input.LatePolicy.LateSubmissionDeductionEnabled != nil {
		policy.LateSubmissionDeductionEnabled = *input.LatePolicy.LateSubmissionDeductionEnabled
	}
	if input.LatePolicy.LateSubmissionDeduction != nil {
		policy.LateSubmissionDeduction = *input.LatePolicy.LateSubmissionDeduction
	}
	if input.LatePolicy.LateSubmissionInterval != nil {
		policy.LateSubmissionInterval = *input.LatePolicy.LateSubmissionInterval
	}
	if input.LatePolicy.LateSubmissionMinimumPercentEnabled != nil {
		policy.LateSubmissionMinimumPercentEnabled = *input.LatePolicy.LateSubmissionMinimumPercentEnabled
	}
	if input.LatePolicy.LateSubmissionMinimumPercent != nil {
		policy.LateSubmissionMinimumPercent = *input.LatePolicy.LateSubmissionMinimumPercent
	}

	if err := h.latePolicyService.Update(c.Context(), policy); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"late_policy": latePolicyToJSON(policy)})
}

func (h *LatePolicyHandler) DeleteLatePolicy(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	if err := h.latePolicyService.Delete(c.Context(), uint(courseID)); err != nil {
		return responses.InternalError(c, "Could not delete late policy")
	}

	return c.JSON(fiber.Map{"delete": true})
}
