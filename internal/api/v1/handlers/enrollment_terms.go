package handlers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type EnrollmentTermHandler struct {
	termService *service.EnrollmentTermService
}

func NewEnrollmentTermHandler(termService *service.EnrollmentTermService) *EnrollmentTermHandler {
	return &EnrollmentTermHandler{termService: termService}
}

func enrollmentTermToJSON(term *models.EnrollmentTerm) fiber.Map {
	return fiber.Map{
		"id":                      term.ID,
		"account_id":              term.AccountID,
		"name":                    term.Name,
		"sis_term_id":             term.SISTermID,
		"start_at":                term.StartAt,
		"end_at":                  term.EndAt,
		"grading_period_group_id": term.GradingPeriodGroupID,
		"workflow_state":          term.WorkflowState,
		"created_at":              term.CreatedAt,
		"updated_at":              term.UpdatedAt,
	}
}

// termAccountScope parses :account_id and enforces that it is the
// caller's own tenant (super_admin excepted). wrote=true means a 400/404
// was written and the caller MUST short-circuit. By-id lookups are then
// tied to this account, so another tenant's term id is a 404.
func termAccountScope(c *fiber.Ctx) (uint, bool) {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		_ = responses.BadRequest(c, "Invalid account ID")
		return 0, true
	}
	if assertSameTenant(c, uint(accountID)) {
		return 0, true
	}
	return uint(accountID), false
}

func termWriteError(c *fiber.Ctx, err error) error {
	if errors.Is(err, service.ErrSISTermIDTaken) {
		return responses.Error(c, fiber.StatusConflict, err.Error())
	}
	return responses.BadRequest(c, err.Error())
}

func (h *EnrollmentTermHandler) ListTerms(c *fiber.Ctx) error {
	accountID, wrote := termAccountScope(c)
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.termService.ListTerms(c.Context(), accountID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch enrollment terms")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	terms := make([]fiber.Map, len(result.Items))
	for i, t := range result.Items {
		terms[i] = enrollmentTermToJSON(&t)
	}

	return c.JSON(fiber.Map{"enrollment_terms": terms})
}

func (h *EnrollmentTermHandler) CreateTerm(c *fiber.Ctx) error {
	accountID, wrote := termAccountScope(c)
	if wrote {
		return nil
	}

	var input struct {
		EnrollmentTerm struct {
			Name                 string  `json:"name"`
			SISTermID            *string `json:"sis_term_id"`
			StartAt              *string `json:"start_at"`
			EndAt                *string `json:"end_at"`
			GradingPeriodGroupID *uint   `json:"grading_period_group_id"`
		} `json:"enrollment_term"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	term := &models.EnrollmentTerm{
		AccountID:            accountID,
		Name:                 input.EnrollmentTerm.Name,
		SISTermID:            service.NormalizeSISTermID(input.EnrollmentTerm.SISTermID),
		GradingPeriodGroupID: input.EnrollmentTerm.GradingPeriodGroupID,
	}

	if input.EnrollmentTerm.StartAt != nil && *input.EnrollmentTerm.StartAt != "" {
		t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.StartAt)
		if err != nil {
			return responses.BadRequest(c, "Invalid start_at date format, use RFC3339")
		}
		term.StartAt = &t
	}

	if input.EnrollmentTerm.EndAt != nil && *input.EnrollmentTerm.EndAt != "" {
		t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.EndAt)
		if err != nil {
			return responses.BadRequest(c, "Invalid end_at date format, use RFC3339")
		}
		term.EndAt = &t
	}

	if err := h.termService.CreateTerm(c.Context(), term); err != nil {
		return termWriteError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(enrollmentTermToJSON(term))
}

func (h *EnrollmentTermHandler) GetTerm(c *fiber.Ctx) error {
	accountID, wrote := termAccountScope(c)
	if wrote {
		return nil
	}

	termID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid term ID")
	}

	term, err := h.termService.GetTerm(c.Context(), uint(termID), accountID)
	if err != nil {
		return responses.NotFound(c, "enrollment term")
	}

	return c.JSON(enrollmentTermToJSON(term))
}

func (h *EnrollmentTermHandler) UpdateTerm(c *fiber.Ctx) error {
	accountID, wrote := termAccountScope(c)
	if wrote {
		return nil
	}

	termID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid term ID")
	}

	term, err := h.termService.GetTerm(c.Context(), uint(termID), accountID)
	if err != nil {
		return responses.NotFound(c, "enrollment term")
	}

	var input struct {
		EnrollmentTerm struct {
			Name                 *string `json:"name"`
			SISTermID            *string `json:"sis_term_id"`
			StartAt              *string `json:"start_at"`
			EndAt                *string `json:"end_at"`
			GradingPeriodGroupID *uint   `json:"grading_period_group_id"`
		} `json:"enrollment_term"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.EnrollmentTerm.Name != nil {
		term.Name = *input.EnrollmentTerm.Name
	}
	if input.EnrollmentTerm.SISTermID != nil {
		// "" clears the SIS id (stored as NULL).
		term.SISTermID = service.NormalizeSISTermID(input.EnrollmentTerm.SISTermID)
	}
	if input.EnrollmentTerm.GradingPeriodGroupID != nil {
		term.GradingPeriodGroupID = input.EnrollmentTerm.GradingPeriodGroupID
	}
	if input.EnrollmentTerm.StartAt != nil {
		if *input.EnrollmentTerm.StartAt == "" {
			term.StartAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.StartAt)
			if err != nil {
				return responses.BadRequest(c, "Invalid start_at date format, use RFC3339")
			}
			term.StartAt = &t
		}
	}
	if input.EnrollmentTerm.EndAt != nil {
		if *input.EnrollmentTerm.EndAt == "" {
			term.EndAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *input.EnrollmentTerm.EndAt)
			if err != nil {
				return responses.BadRequest(c, "Invalid end_at date format, use RFC3339")
			}
			term.EndAt = &t
		}
	}

	if err := h.termService.UpdateTerm(c.Context(), term); err != nil {
		return termWriteError(c, err)
	}

	return c.JSON(enrollmentTermToJSON(term))
}

func (h *EnrollmentTermHandler) DeleteTerm(c *fiber.Ctx) error {
	accountID, wrote := termAccountScope(c)
	if wrote {
		return nil
	}

	termID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid term ID")
	}

	if _, err := h.termService.GetTerm(c.Context(), uint(termID), accountID); err != nil {
		return responses.NotFound(c, "enrollment term")
	}

	if err := h.termService.DeleteTerm(c.Context(), uint(termID), accountID); err != nil {
		return responses.InternalError(c, "Could not delete enrollment term")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *EnrollmentTermHandler) GetCurrentTerm(c *fiber.Ctx) error {
	accountID, wrote := termAccountScope(c)
	if wrote {
		return nil
	}

	term, err := h.termService.GetCurrentTerm(c.Context(), accountID)
	if err != nil {
		return responses.NotFound(c, "current enrollment term")
	}

	return c.JSON(enrollmentTermToJSON(term))
}
