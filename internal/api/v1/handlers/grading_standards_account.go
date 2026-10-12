package handlers

// Account-level grading schemes (G039): schemes an admin defines once
// for every course of the account, plus the shared input validation for
// course and account schemes (sorted, unique, starting at 0%, optional
// points-based display).

import (
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// gradingStandardInput is the create / update body.
type gradingStandardInput struct {
	GradingStandard struct {
		Title         *string         `json:"title"`
		Data          json.RawMessage `json:"data"`
		PointsBased   *bool           `json:"points_based"`
		ScalingFactor *float64        `json:"scaling_factor"`
	} `json:"grading_standard"`
}

// applyGradingStandardInput validates the input onto gs. creating
// requires a title and data. Returns a client error message, or "".
func applyGradingStandardInput(gs *models.GradingStandard, in gradingStandardInput, creating bool) string {
	if in.GradingStandard.Title != nil {
		title := service.SanitizePlainText(strings.TrimSpace(*in.GradingStandard.Title))
		if title == "" {
			return "Grading standard title is required"
		}
		if len(title) > 255 {
			return "Grading standard title is too long"
		}
		gs.Title = title
	} else if creating {
		return "Grading standard title is required"
	}
	raw := strings.TrimSpace(string(in.GradingStandard.Data))
	if raw != "" && raw != "null" {
		data, _, err := service.NormalizeSchemeData(in.GradingStandard.Data)
		if err != nil {
			if errors.Is(err, service.ErrInvalidScheme) {
				return err.Error()
			}
			return "Invalid grading scheme data"
		}
		gs.Data = data
	} else if creating {
		return "Grading standard data is required"
	}
	if in.GradingStandard.PointsBased != nil {
		gs.PointsBased = *in.GradingStandard.PointsBased
	}
	if in.GradingStandard.ScalingFactor != nil {
		f := *in.GradingStandard.ScalingFactor
		if math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 || f > 1000 {
			return "scaling_factor must be a positive number"
		}
		gs.ScalingFactor = f
	}
	if gs.ScalingFactor <= 0 {
		gs.ScalingFactor = 1
	}
	return ""
}

// loadAccountStandard loads :id as a scheme of :account_id (already
// tenant-checked by RequireAdmin). On a miss it writes a 404 and returns
// wrote=true.
func (h *GradingStandardHandler) loadAccountStandard(c *fiber.Ctx) (*models.GradingStandard, bool) {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		_ = responses.BadRequest(c, "Invalid account ID")
		return nil, true
	}
	if assertSameTenant(c, uint(accountID)) {
		return nil, true
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid grading standard ID")
		return nil, true
	}
	gs, err := h.repo.FindByIDInAccount(c.Context(), uint(id), callerAccountID(c))
	if err != nil || gs == nil || gs.ContextType != "Account" || gs.ContextID != uint(accountID) || gs.WorkflowState == "deleted" {
		_ = responses.NotFound(c, "grading standard")
		return nil, true
	}
	return gs, false
}

// ListAccountGradingStandards handles GET /accounts/:account_id/grading_standards.
func (h *GradingStandardHandler) ListAccountGradingStandards(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if assertSameTenant(c, uint(accountID)) {
		return nil
	}
	standards, err := h.repo.ListByAccount(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch grading standards")
	}
	out := make([]fiber.Map, len(standards))
	for i := range standards {
		out[i] = gradingStandardToJSON(&standards[i])
	}
	return c.JSON(out)
}

// CreateAccountGradingStandard handles POST /accounts/:account_id/grading_standards.
func (h *GradingStandardHandler) CreateAccountGradingStandard(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if assertSameTenant(c, uint(accountID)) {
		return nil
	}
	var in gradingStandardInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	gs := &models.GradingStandard{ContextType: "Account", ContextID: uint(accountID), WorkflowState: "active"}
	if msg := applyGradingStandardInput(gs, in, true); msg != "" {
		return responses.BadRequest(c, msg)
	}
	if err := h.repo.Create(c.Context(), gs); err != nil {
		return responses.InternalError(c, "Could not create grading standard")
	}
	return c.Status(fiber.StatusCreated).JSON(gradingStandardToJSON(gs))
}

// UpdateAccountGradingStandard handles PUT /accounts/:account_id/grading_standards/:id.
func (h *GradingStandardHandler) UpdateAccountGradingStandard(c *fiber.Ctx) error {
	gs, wrote := h.loadAccountStandard(c)
	if wrote {
		return nil
	}
	var in gradingStandardInput
	if err := c.BodyParser(&in); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if msg := applyGradingStandardInput(gs, in, false); msg != "" {
		return responses.BadRequest(c, msg)
	}
	if err := h.repo.Update(c.Context(), gs); err != nil {
		return responses.InternalError(c, "Could not update grading standard")
	}
	return c.JSON(gradingStandardToJSON(gs))
}

// DeleteAccountGradingStandard handles DELETE /accounts/:account_id/grading_standards/:id.
func (h *GradingStandardHandler) DeleteAccountGradingStandard(c *fiber.Ctx) error {
	gs, wrote := h.loadAccountStandard(c)
	if wrote {
		return nil
	}
	if err := h.repo.Delete(c.Context(), gs.ID); err != nil {
		return responses.InternalError(c, "Could not delete grading standard")
	}
	return c.JSON(fiber.Map{"delete": true})
}
